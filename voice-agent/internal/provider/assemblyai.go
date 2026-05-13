package provider

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"

	"callhub/voice-agent/internal/config"
	"callhub/voice-agent/internal/session"
)

// AssemblyAI uses AssemblyAI Universal Streaming for ASR, OpenAI Chat Completions for LLM
// reasoning, and OpenAI's speech endpoint for TTS by default. All three components are pluggable
// via ASSEMBLYAI_LLM_URL / ASSEMBLYAI_TTS_URL.
//
// Spec: https://www.assemblyai.com/docs/speech-to-text/universal-streaming
type AssemblyAI struct {
	cfg    config.AssemblyAIConfig
	logger *slog.Logger
	http   *http.Client
}

func NewAssemblyAI(cfg config.AssemblyAIConfig, logger *slog.Logger) *AssemblyAI {
	return &AssemblyAI{cfg: cfg, logger: logger, http: &http.Client{Timeout: 30 * time.Second}}
}

func (a *AssemblyAI) Name() string { return "assemblyai" }

func (a *AssemblyAI) Run(ctx context.Context, s *session.Session) error {
	if a.cfg.APIKey == "" {
		emit(s, session.Event{Type: "error", Message: "assemblyai api key not configured"})
		return ErrNotConfigured
	}
	if a.cfg.LLMKey == "" {
		emit(s, session.Event{Type: "error", Message: "assemblyai llm key not configured"})
		return ErrNotConfigured
	}

	conv := newConversation(SystemPrompt(s.Config))

	// AssemblyAI Universal Streaming WebSocket. Auth is via temporary token returned by their
	// REST API; for simplicity we use the static API key as a header (works for v3 streaming).
	u, _ := url.Parse("wss://streaming.assemblyai.com/v3/ws")
	q := u.Query()
	q.Set("sample_rate", "16000")
	q.Set("encoding", "pcm_s16le")
	q.Set("format_turns", "true")
	u.RawQuery = q.Encode()

	headers := http.Header{}
	headers.Set("Authorization", a.cfg.APIKey)
	conn, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "assemblyai_dial: " + err.Error()})
		return fmt.Errorf("assemblyai dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")

	emit(s, session.Event{Type: "status", State: "ready"})

	// Audio pump: AssemblyAI v3 accepts base64 in JSON or raw binary; we use binary.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-s.AudioIn:
				if !ok {
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"Terminate"}`))
					return
				}
				_ = conn.Write(ctx, websocket.MessageBinary, chunk)
			}
		}
	}()

	var pipelineMu sync.Mutex
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg assemblyaiMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "Begin":
			emit(s, session.Event{Type: "status", State: "listening"})
		case "Turn":
			if msg.Transcript == "" {
				continue
			}
			if !msg.EndOfTurn {
				emit(s, session.Event{Type: "transcript", Role: "user", Text: msg.Transcript, Final: false})
				continue
			}
			text := msg.Transcript
			emit(s, session.Event{Type: "transcript", Role: "user", Text: text, Final: true})
			s.AppendTranscript("user", text)
			conv.Add("user", text)

			go func(turn []chatMessage) {
				pipelineMu.Lock()
				defer pipelineMu.Unlock()
				emit(s, session.Event{Type: "status", State: "thinking"})
				reply, err := a.completeLLM(ctx, turn)
				if err != nil {
					a.logger.Warn("assemblyai llm", "error", err)
					emit(s, session.Event{Type: "error", Message: "llm: " + err.Error()})
					return
				}
				conv.Add("assistant", reply)
				emit(s, session.Event{Type: "transcript", Role: "agent", Text: reply, Final: true})
				s.AppendTranscript("agent", reply)

				emit(s, session.Event{Type: "status", State: "speaking"})
				audio, err := a.synthesize(ctx, reply, s.Config.Voice)
				if err != nil {
					a.logger.Warn("assemblyai tts", "error", err)
					emit(s, session.Event{Type: "error", Message: "tts: " + err.Error()})
					return
				}
				select {
				case s.AudioOut <- audio:
				case <-ctx.Done():
				}
				emit(s, session.Event{Type: "status", State: "listening"})
			}(conv.Snapshot())
		}
	}
}

type assemblyaiMessage struct {
	Type       string `json:"type"`
	Transcript string `json:"transcript"`
	EndOfTurn  bool   `json:"end_of_turn"`
	TurnOrder  int    `json:"turn_order"`
}

func (a *AssemblyAI) completeLLM(ctx context.Context, messages []chatMessage) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    a.cfg.LLMModel,
		"messages": messages,
		"stream":   false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.LLMURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.LLMKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("llm %d: %s", resp.StatusCode, string(respBody))
	}
	var parsed chatCompletionResponse
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", err
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("empty llm response")
	}
	return parsed.Choices[0].Message.Content, nil
}

// synthesize calls the OpenAI-compatible speech endpoint and returns PCM16 audio bytes.
// We request the wav container (16kHz mono PCM16) and strip the 44-byte RIFF header.
func (a *AssemblyAI) synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		voice = a.cfg.TTSVoice
	}
	body, _ := json.Marshal(map[string]any{
		"model":           a.cfg.TTSModel,
		"voice":           voice,
		"input":           text,
		"response_format": "wav",
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, a.cfg.TTSURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+a.cfg.TTSKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := a.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts %d: %s", resp.StatusCode, string(b))
	}
	wavBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// Strip the 44-byte WAV header if present so downstream sees raw PCM16.
	if len(wavBytes) > 44 && bytes.HasPrefix(wavBytes, []byte("RIFF")) {
		return wavBytes[44:], nil
	}
	return wavBytes, nil
}

// Avoid unused-import errors if the future moves things around.
var _ = base64.StdEncoding

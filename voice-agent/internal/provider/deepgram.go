package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// Deepgram composes three services to form a full voice agent:
//   1. ASR: Deepgram Live (WebSocket) - audio in → transcripts
//   2. LLM: OpenAI Chat Completions (HTTP) - text → text
//   3. TTS: Deepgram Aura (HTTP) - text → audio
//
// Each user turn (final transcript) triggers an LLM call, whose response is synthesized and
// piped back as audio to the caller.
type Deepgram struct {
	cfg    config.DeepgramConfig
	logger *slog.Logger
	http   *http.Client
}

func NewDeepgram(cfg config.DeepgramConfig, logger *slog.Logger) *Deepgram {
	return &Deepgram{cfg: cfg, logger: logger, http: &http.Client{Timeout: 30 * time.Second}}
}

func (d *Deepgram) Name() string { return "deepgram" }

func (d *Deepgram) Run(ctx context.Context, s *session.Session) error {
	if d.cfg.APIKey == "" {
		emit(s, session.Event{Type: "error", Message: "deepgram api key not configured"})
		return ErrNotConfigured
	}
	if d.cfg.LLMKey == "" {
		emit(s, session.Event{Type: "error", Message: "deepgram llm key not configured (set DEEPGRAM_LLM_KEY or OPENAI_API_KEY)"})
		return ErrNotConfigured
	}

	conv := newConversation(SystemPrompt(s.Config))

	// 1) Open ASR live socket.
	asrURL := buildDeepgramASRURL(d.cfg.ASRModel, s.Config.Language)
	headers := http.Header{}
	headers.Set("Authorization", "Token "+d.cfg.APIKey)
	conn, _, err := websocket.Dial(ctx, asrURL, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "deepgram_dial: " + err.Error()})
		return fmt.Errorf("deepgram dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")

	emit(s, session.Event{Type: "status", State: "ready"})

	// 2) Pump audio frames to Deepgram ASR.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-s.AudioIn:
				if !ok {
					_ = conn.Write(ctx, websocket.MessageText, []byte(`{"type":"CloseStream"}`))
					return
				}
				_ = conn.Write(ctx, websocket.MessageBinary, chunk)
			}
		}
	}()

	// 3) Read transcripts; on each final, dispatch to LLM → TTS pipeline.
	var (
		pipelineMu sync.Mutex
	)
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg deepgramASRMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type == "Results" {
			text := msg.Transcript()
			if text == "" {
				continue
			}
			if !msg.IsFinal {
				emit(s, session.Event{Type: "transcript", Role: "user", Text: text, Final: false})
				continue
			}
			emit(s, session.Event{Type: "transcript", Role: "user", Text: text, Final: true})
			s.AppendTranscript("user", text)
			conv.Add("user", text)

			// Serialize pipeline turns: only one LLM+TTS at a time. New finals during a response
			// could implement barge-in by cancelling the running TTS — left as future work.
			go func(turn []chatMessage) {
				pipelineMu.Lock()
				defer pipelineMu.Unlock()
				emit(s, session.Event{Type: "status", State: "thinking"})
				reply, err := d.completeLLM(ctx, turn)
				if err != nil {
					d.logger.Warn("deepgram llm", "error", err)
					emit(s, session.Event{Type: "error", Message: "llm: " + err.Error()})
					return
				}
				conv.Add("assistant", reply)
				emit(s, session.Event{Type: "transcript", Role: "agent", Text: reply, Final: true})
				s.AppendTranscript("agent", reply)

				emit(s, session.Event{Type: "status", State: "speaking"})
				audio, err := d.synthesize(ctx, reply, s.Config.Voice)
				if err != nil {
					d.logger.Warn("deepgram tts", "error", err)
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

func buildDeepgramASRURL(model, language string) string {
	u, _ := url.Parse("wss://api.deepgram.com/v1/listen")
	q := u.Query()
	q.Set("model", model)
	q.Set("encoding", "linear16")
	q.Set("sample_rate", "16000")
	q.Set("channels", "1")
	q.Set("smart_format", "true")
	q.Set("interim_results", "true")
	q.Set("vad_events", "true")
	q.Set("endpointing", "400")
	if language != "" {
		q.Set("language", language)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

type deepgramASRMessage struct {
	Type    string `json:"type"`
	IsFinal bool   `json:"is_final"`
	Channel struct {
		Alternatives []struct {
			Transcript string `json:"transcript"`
		} `json:"alternatives"`
	} `json:"channel"`
}

func (m deepgramASRMessage) Transcript() string {
	if len(m.Channel.Alternatives) > 0 {
		return m.Channel.Alternatives[0].Transcript
	}
	return ""
}

// completeLLM calls the configured chat completion endpoint with the running conversation.
func (d *Deepgram) completeLLM(ctx context.Context, messages []chatMessage) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":    d.cfg.LLMModel,
		"messages": messages,
		"stream":   false,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.cfg.LLMURL, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+d.cfg.LLMKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
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

// synthesize calls Deepgram Aura with the agent's text and returns PCM16 audio bytes.
func (d *Deepgram) synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		voice = d.cfg.TTSModel
	}
	u, _ := url.Parse("https://api.deepgram.com/v1/speak")
	q := u.Query()
	q.Set("model", d.cfg.TTSModel)
	q.Set("encoding", "linear16")
	q.Set("sample_rate", "16000")
	u.RawQuery = q.Encode()

	body, _ := json.Marshal(map[string]string{"text": text})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u.String(), bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Token "+d.cfg.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("tts %d: %s", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// chatMessage is the OpenAI-compatible message shape used by both Deepgram and AssemblyAI pipelines.
type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
}

// conversation keeps a bounded LLM history with the system prompt always pinned.
type conversation struct {
	mu       sync.Mutex
	messages []chatMessage
	maxTurns int
}

func newConversation(systemPrompt string) *conversation {
	return &conversation{
		messages: []chatMessage{{Role: "system", Content: systemPrompt}},
		maxTurns: 16, // keep history small to control LLM cost
	}
}

func (c *conversation) Add(role, content string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messages = append(c.messages, chatMessage{Role: role, Content: content})
	// Truncate non-system messages.
	if len(c.messages) > c.maxTurns+1 {
		kept := append([]chatMessage{c.messages[0]}, c.messages[len(c.messages)-c.maxTurns:]...)
		c.messages = kept
	}
}

func (c *conversation) Snapshot() []chatMessage {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]chatMessage, len(c.messages))
	copy(out, c.messages)
	return out
}

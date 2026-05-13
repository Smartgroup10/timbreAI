package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// OpenAIRealtime wraps the OpenAI Realtime WebSocket API: a single connection that does
// ASR + LLM + TTS together. Audio is PCM16 LE 24kHz on the wire (Realtime API spec).
//
// Spec: https://platform.openai.com/docs/guides/realtime
type OpenAIRealtime struct {
	cfg    config.OpenAIConfig
	logger *slog.Logger
}

func NewOpenAIRealtime(cfg config.OpenAIConfig, logger *slog.Logger) *OpenAIRealtime {
	return &OpenAIRealtime{cfg: cfg, logger: logger}
}

func (o *OpenAIRealtime) Name() string { return "openai_realtime" }

func (o *OpenAIRealtime) Run(ctx context.Context, s *session.Session) error {
	// Per-tenant credentials override the env defaults.
	apiKey := pick(s.Config.Credentials.OpenAIAPIKey, o.cfg.APIKey)
	model := pick(s.Config.Credentials.OpenAIRealtimeModel, o.cfg.Model)
	if apiKey == "" {
		emit(s, session.Event{Type: "error", Message: "openai api key not configured (tenant or env)"})
		return ErrNotConfigured
	}

	url := "wss://api.openai.com/v1/realtime?model=" + model
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	headers.Set("OpenAI-Beta", "realtime=v1")

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "openai_dial: " + err.Error()})
		return fmt.Errorf("openai realtime dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")

	voice := pick(s.Config.Voice, pick(s.Config.Credentials.OpenAIRealtimeVoice, o.cfg.Voice))

	// Configure session: PCM16 24kHz audio in/out, instructions, voice, server VAD for turn-taking.
	initMsg := map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"modalities":         []string{"audio", "text"},
			"instructions":       SystemPrompt(s.Config),
			"voice":              voice,
			"input_audio_format":  "pcm16",
			"output_audio_format": "pcm16",
			"turn_detection": map[string]any{
				"type":              "server_vad",
				"threshold":         0.5,
				"prefix_padding_ms": 300,
				"silence_duration_ms": 600,
			},
			"input_audio_transcription": map[string]any{"model": "whisper-1"},
		},
	}
	if err := writeJSON(ctx, conn, initMsg); err != nil {
		return err
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// Goroutine: pump audio frames from session.AudioIn → OpenAI as input_audio_buffer.append.
	go o.pumpAudioIn(ctx, conn, s)

	// Main loop: read JSON events from OpenAI, fan out audio_out + transcript events.
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		var msg map[string]any
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		o.handleEvent(msg, s)
	}
}

func (o *OpenAIRealtime) pumpAudioIn(ctx context.Context, conn *websocket.Conn, s *session.Session) {
	for {
		select {
		case <-ctx.Done():
			return
		case chunk, ok := <-s.AudioIn:
			if !ok {
				_ = writeJSON(ctx, conn, map[string]any{"type": "input_audio_buffer.commit"})
				return
			}
			if len(chunk) == 0 {
				continue
			}
			_ = writeJSON(ctx, conn, map[string]any{
				"type":  "input_audio_buffer.append",
				"audio": base64.StdEncoding.EncodeToString(chunk),
			})
		}
	}
}

func (o *OpenAIRealtime) handleEvent(msg map[string]any, s *session.Session) {
	t, _ := msg["type"].(string)
	switch t {
	case "response.audio.delta":
		// base64 PCM16 24kHz mono.
		if data, _ := msg["delta"].(string); data != "" {
			if audio, err := base64.StdEncoding.DecodeString(data); err == nil {
				select {
				case s.AudioOut <- audio:
				case <-s.Context().Done():
				}
			}
		}
	case "response.audio_transcript.delta":
		if delta, _ := msg["delta"].(string); delta != "" {
			emit(s, session.Event{Type: "transcript", Role: "agent", Text: delta, Final: false})
		}
	case "response.audio_transcript.done":
		if txt, _ := msg["transcript"].(string); txt != "" {
			emit(s, session.Event{Type: "transcript", Role: "agent", Text: txt, Final: true})
			s.AppendTranscript("agent", txt)
		}
	case "conversation.item.input_audio_transcription.completed":
		if txt, _ := msg["transcript"].(string); txt != "" {
			emit(s, session.Event{Type: "transcript", Role: "user", Text: txt, Final: true})
			s.AppendTranscript("user", txt)
		}
	case "input_audio_buffer.speech_started":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "input_audio_buffer.speech_stopped":
		emit(s, session.Event{Type: "status", State: "thinking"})
	case "response.created":
		emit(s, session.Event{Type: "status", State: "speaking"})
	case "response.done":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "error":
		emit(s, session.Event{Type: "error", Message: jsonString(msg, "error", "message")})
	}
}

func jsonString(m map[string]any, keys ...string) string {
	var v any = m
	for _, k := range keys {
		obj, ok := v.(map[string]any)
		if !ok {
			return ""
		}
		v = obj[k]
	}
	if s, ok := v.(string); ok {
		return s
	}
	return ""
}

func writeJSON(ctx context.Context, conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return conn.Write(ctx, websocket.MessageText, data)
}

package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// Deepgram talks to the Voice Agent WebSocket API end-to-end:
//   wss://agent.deepgram.com/v1/agent/converse
//
// One socket does everything: listen (ASR), think (LLM hosted by Deepgram pointing at any vendor
// model), speak (TTS via Aura). Input audio is linear16 binary frames; output audio is binary
// frames at the configured sample rate. JSON text frames carry events (ConversationText,
// AgentThinking, UserStartedSpeaking, etc).
//
// Spec: https://developers.deepgram.com/reference/voice-agent/voice-agent
type Deepgram struct {
	cfg    config.DeepgramConfig
	logger *slog.Logger
}

func NewDeepgram(cfg config.DeepgramConfig, logger *slog.Logger) *Deepgram {
	return &Deepgram{cfg: cfg, logger: logger}
}

func (d *Deepgram) Name() string { return "deepgram" }

const deepgramSampleRate = 16000

func (d *Deepgram) Run(ctx context.Context, s *session.Session) error {
	apiKey := pick(s.Config.Credentials.DeepgramAPIKey, d.cfg.APIKey)
	listenModel := pick(s.Config.Credentials.DeepgramListenModel, d.cfg.ListenModel)
	thinkProvider := pick(s.Config.Credentials.DeepgramThinkProvider, d.cfg.ThinkProvider)
	thinkModel := pick(s.Config.Credentials.DeepgramThinkModel, d.cfg.ThinkModel)
	speakModel := pick(s.Config.Credentials.DeepgramSpeakModel, d.cfg.SpeakModel)
	greeting := pick(s.Config.Credentials.DeepgramGreeting, d.cfg.Greeting)

	if apiKey == "" {
		emit(s, session.Event{Type: "error", Message: "deepgram api key not configured (tenant or env)"})
		return ErrNotConfigured
	}

	headers := http.Header{}
	headers.Set("Authorization", "Token "+apiKey)
	conn, _, err := websocket.Dial(ctx, "wss://agent.deepgram.com/v1/agent/converse",
		&websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "deepgram_dial: " + err.Error()})
		return fmt.Errorf("deepgram voice agent dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")

	// Settings: full agent configuration in a single message.
	settings := map[string]any{
		"type": "Settings",
		"audio": map[string]any{
			"input": map[string]any{
				"encoding":    "linear16",
				"sample_rate": deepgramSampleRate,
			},
			"output": map[string]any{
				"encoding":    "linear16",
				"sample_rate": deepgramSampleRate,
				"container":   "none",
			},
		},
		"agent": map[string]any{
			"listen": map[string]any{
				"provider": map[string]any{
					"type":  "deepgram",
					"model": listenModel,
				},
			},
			"think": map[string]any{
				"provider": map[string]any{
					"type":  thinkProvider,
					"model": thinkModel,
				},
				"prompt": SystemPrompt(s.Config),
			},
			"speak": map[string]any{
				"provider": map[string]any{
					"type":  "deepgram",
					"model": speakModel,
				},
			},
			"greeting": greeting,
		},
	}
	if err := writeJSON(ctx, conn, settings); err != nil {
		return err
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// Goroutine: pump caller audio (binary PCM16) to Deepgram.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-s.AudioIn:
				if !ok {
					return
				}
				_ = conn.Write(ctx, websocket.MessageBinary, chunk)
			}
		}
	}()

	// Main loop: read both binary audio frames (agent speaking) and text events (transcripts,
	// status changes).
	for {
		msgType, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		switch msgType {
		case websocket.MessageBinary:
			// Agent audio chunk — forward straight to the session out channel.
			cp := make([]byte, len(raw))
			copy(cp, raw)
			select {
			case s.AudioOut <- cp:
			case <-ctx.Done():
				return ctx.Err()
			}
		case websocket.MessageText:
			d.handleEvent(raw, s)
		}
	}
}

func (d *Deepgram) handleEvent(raw []byte, s *session.Session) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	switch probe.Type {
	case "Welcome", "SettingsApplied":
		// no-op; could surface as status if we wanted
	case "UserStartedSpeaking":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "AgentThinking":
		emit(s, session.Event{Type: "status", State: "thinking"})
	case "AgentStartedSpeaking":
		emit(s, session.Event{Type: "status", State: "speaking"})
	case "AgentAudioDone":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "ConversationText":
		// { role: "user" | "assistant", content: "..." }
		var ev struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Content != "" {
			out := "agent"
			if ev.Role == "user" {
				out = "user"
			}
			emit(s, session.Event{Type: "transcript", Role: out, Text: ev.Content, Final: true})
			s.AppendTranscript(out, ev.Content)
		}
	case "Error":
		var ev struct {
			Description string `json:"description"`
			Code        string `json:"code"`
		}
		_ = json.Unmarshal(raw, &ev)
		d.logger.Warn("deepgram agent error", "code", ev.Code, "desc", ev.Description)
		emit(s, session.Event{Type: "error", Message: ev.Description})
	case "Warning":
		var ev struct {
			Description string `json:"description"`
		}
		_ = json.Unmarshal(raw, &ev)
		d.logger.Info("deepgram agent warning", "desc", ev.Description)
	}
}

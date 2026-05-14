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

// AssemblyAI talks to the Voice Agent WebSocket API end-to-end:
//   wss://agents.assemblyai.com/v1/ws
//
// AssemblyAI hosts ASR + LLM + TTS internally. We pick the voice and pass a system prompt; they
// drive turn-taking and barge-in semantically. Input/output audio is base64-encoded PCM16
// embedded in JSON frames (not raw binary like Deepgram).
//
// Spec: https://www.assemblyai.com/docs/voice-agents/voice-agent-api
type AssemblyAI struct {
	cfg    config.AssemblyAIConfig
	logger *slog.Logger
}

func NewAssemblyAI(cfg config.AssemblyAIConfig, logger *slog.Logger) *AssemblyAI {
	return &AssemblyAI{cfg: cfg, logger: logger}
}

func (a *AssemblyAI) Name() string { return "assemblyai" }

func (a *AssemblyAI) Run(ctx context.Context, s *session.Session) error {
	apiKey := pick(s.Config.Credentials.AssemblyAIAPIKey, a.cfg.APIKey)
	voice := pick(s.Config.Credentials.AssemblyAIVoice, a.cfg.Voice)
	greeting := pick(s.Config.Credentials.AssemblyAIGreeting, a.cfg.Greeting)

	if apiKey == "" {
		emit(s, session.Event{Type: "error", Message: "assemblyai api key not configured (tenant or env)"})
		return ErrNotConfigured
	}

	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+apiKey)
	conn, _, err := websocket.Dial(ctx, "wss://agents.assemblyai.com/v1/ws",
		&websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "assemblyai_dial: " + err.Error()})
		return fmt.Errorf("assemblyai voice agent dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")
	// 32 KB default es muy poco para audio TTS — fix mismo patrón que OpenAI.
	conn.SetReadLimit(10 * 1024 * 1024)

	// session.update with everything the agent needs.
	settings := map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"system_prompt": SystemPrompt(s.Config),
			"greeting":      greeting,
			"output": map[string]any{
				"voice": voice,
			},
		},
	}
	if err := writeJSON(ctx, conn, settings); err != nil {
		return err
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// Goroutine: pump caller audio to AssemblyAI as base64 in input.audio events.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-s.AudioIn:
				if !ok {
					return
				}
				_ = writeJSON(ctx, conn, map[string]any{
					"type":  "input.audio",
					"audio": base64.StdEncoding.EncodeToString(chunk),
				})
			}
		}
	}()

	// Main loop: read JSON events, decode audio replies inline.
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		a.handleEvent(raw, s)
	}
}

func (a *AssemblyAI) handleEvent(raw []byte, s *session.Session) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	switch probe.Type {
	case "session.ready":
		// {session_id: ...}
	case "input.speech.started":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "input.speech.stopped":
		emit(s, session.Event{Type: "status", State: "thinking"})
	case "transcript.user", "transcript.user.delta":
		var ev struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Text != "" {
			final := probe.Type == "transcript.user"
			emit(s, session.Event{Type: "transcript", Role: "user", Text: ev.Text, Final: final})
			if final {
				s.AppendTranscript("user", ev.Text)
			}
		}
	case "transcript.agent":
		var ev struct {
			Text        string `json:"text"`
			Interrupted bool   `json:"interrupted"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Text != "" {
			emit(s, session.Event{Type: "transcript", Role: "agent", Text: ev.Text, Final: true})
			s.AppendTranscript("agent", ev.Text)
		}
	case "reply.started":
		emit(s, session.Event{Type: "status", State: "speaking"})
	case "reply.audio":
		var ev struct {
			Audio string `json:"audio"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.Audio != "" {
			if pcm, err := base64.StdEncoding.DecodeString(ev.Audio); err == nil {
				select {
				case s.AudioOut <- pcm:
				case <-s.Context().Done():
					return
				}
			}
		}
	case "reply.done":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "session.error":
		var ev struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &ev)
		a.logger.Warn("assemblyai agent error", "msg", ev.Message)
		emit(s, session.Event{Type: "error", Message: ev.Message})
	}
}

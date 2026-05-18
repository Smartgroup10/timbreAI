package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/audio"
	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// ElevenLabs talks to the Conversational AI Agents WebSocket API:
//
//	wss://api.elevenlabs.io/v1/convai/conversation?agent_id=...
//
// El AGENTE está pre-configurado en el dashboard de ElevenLabs: voz,
// system prompt, LLM, tools del lado server. Nosotros conectamos con
// su agent_id y opcionalmente overrideamos algunos parámetros por
// sesión vía conversation_initiation_client_data.
//
// Audio: PCM16 16 kHz mono base64 en JSON (no binario crudo). AudioSocket
// nos da slin 8 kHz, así que hacemos resample en ambos sentidos con
// audio.UpsampleSlin8kTo16k / audio.DownsampleSlin16kTo8k.
//
// Auth: API key en query param para agentes privados, o agent_id solo
// para agentes públicos. Usamos siempre header xi-api-key cuando hay
// key disponible (cobertura para ambos casos).
//
// Spec:
//
//	https://elevenlabs.io/docs/eleven-agents/api-reference/eleven-agents/websocket
type ElevenLabs struct {
	cfg    config.ElevenLabsConfig
	logger *slog.Logger
}

func NewElevenLabs(cfg config.ElevenLabsConfig, logger *slog.Logger) *ElevenLabs {
	return &ElevenLabs{cfg: cfg, logger: logger}
}

func (e *ElevenLabs) Name() string { return "elevenlabs" }

func (e *ElevenLabs) Run(ctx context.Context, s *session.Session) error {
	apiKey := pick(s.Config.Credentials.ElevenLabsAPIKey, e.cfg.APIKey)
	agentID := pick(s.Config.Credentials.ElevenLabsAgentID, e.cfg.AgentID)
	if agentID == "" {
		emit(s, session.Event{Type: "error", Message: "elevenlabs agent_id not configured (bot or env)"})
		return ErrNotConfigured
	}

	url := "wss://api.elevenlabs.io/v1/convai/conversation?agent_id=" + agentID
	headers := http.Header{}
	if apiKey != "" {
		// Header xi-api-key cubre agentes privados. Para públicos,
		// el agent_id en la query basta y este header se ignora.
		headers.Set("xi-api-key", apiKey)
	}

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "elevenlabs_dial: " + err.Error()})
		return fmt.Errorf("elevenlabs dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")
	conn.SetReadLimit(10 * 1024 * 1024)

	// conversation_initiation_client_data permite override del system_prompt
	// y de TTS settings. El agent_id ya trae su config base; aquí solo
	// reemplazamos lo dinámico (lead_name, objective, idioma de respuesta).
	//
	// Schema oficial:
	//   conversation_config_override: { agent: { prompt: { prompt: "..." },
	//                                          first_message, language },
	//                                   tts: { voice_id } }
	//
	// Para no romper la config del operador en el dashboard, solo
	// overrideamos prompt si tenemos uno y dejamos voice/LLM en sus
	// defaults del agente.
	initData := map[string]any{
		"type": "conversation_initiation_client_data",
	}
	overrides := map[string]any{}
	agentOverride := map[string]any{}
	if prompt := SystemPrompt(s.Config); prompt != "" {
		agentOverride["prompt"] = map[string]any{"prompt": prompt}
	}
	if s.Config.Language != "" {
		agentOverride["language"] = elevenlabsLang(s.Config.Language)
	}
	if len(agentOverride) > 0 {
		overrides["agent"] = agentOverride
	}
	if len(overrides) > 0 {
		initData["conversation_config_override"] = overrides
	}
	if err := writeJSON(ctx, conn, initData); err != nil {
		return err
	}
	e.logger.Info("elevenlabs initiation sent",
		"session", s.ID, "agent_id", agentID)

	// Esperamos conversation_initiation_metadata antes de pumpear audio
	// — confirma que el agente arrancó y nos da el sample_rate definitivo.
	if err := e.waitInitMetadata(ctx, conn, 10*time.Second); err != nil {
		emit(s, session.Event{Type: "error", Message: "elevenlabs_metadata: " + err.Error()})
		return fmt.Errorf("elevenlabs init metadata: %w", err)
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// Goroutine: pump caller audio (slin 8k AudioSocket) → upsample 16k
	// → base64 JSON {type: user_audio_chunk}.
	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case chunk, ok := <-s.AudioIn:
				if !ok {
					return
				}
				if len(chunk) == 0 {
					continue
				}
				up := audio.UpsampleSlin8kTo16k(chunk)
				_ = writeJSON(ctx, conn, map[string]any{
					"type":             "user_audio_chunk",
					"user_audio_chunk": base64.StdEncoding.EncodeToString(up),
				})
			}
		}
	}()

	// Main loop: read JSON events.
	for {
		_, raw, err := conn.Read(ctx)
		if err != nil {
			return err
		}
		e.handleEvent(ctx, conn, raw, s)
	}
}

// waitInitMetadata drena el WS hasta encontrar conversation_initiation_metadata,
// abortando si llega un error o se agota el timeout.
func (e *ElevenLabs) waitInitMetadata(ctx context.Context, conn *websocket.Conn, timeout time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		_, raw, err := conn.Read(deadline)
		if err != nil {
			return err
		}
		var probe struct {
			Type    string `json:"type"`
			Message string `json:"message"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		switch probe.Type {
		case "conversation_initiation_metadata":
			return nil
		case "error", "session.error":
			return fmt.Errorf("elevenlabs error: %s", probe.Message)
		}
	}
}

func (e *ElevenLabs) handleEvent(ctx context.Context, conn *websocket.Conn, raw []byte, s *session.Session) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	switch probe.Type {
	case "ping":
		// Mantener viva la conexión: respondemos con pong inmediato.
		// El payload incluye un event_id que debemos echar de vuelta.
		var ping struct {
			PingEvent struct {
				EventID int `json:"event_id"`
			} `json:"ping_event"`
		}
		if err := json.Unmarshal(raw, &ping); err == nil {
			_ = writeJSON(ctx, conn, map[string]any{
				"type":     "pong",
				"event_id": ping.PingEvent.EventID,
			})
		}
	case "vad_score":
		// No-op por ahora; podría servir para barge-in fino.
	case "user_transcript":
		// { user_transcription_event: { user_transcript: "..." } }
		var ev struct {
			UTE struct {
				Text string `json:"user_transcript"`
			} `json:"user_transcription_event"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.UTE.Text != "" {
			emit(s, session.Event{Type: "transcript", Role: "user", Text: ev.UTE.Text, Final: true})
			s.AppendTranscript("user", ev.UTE.Text)
		}
	case "agent_response":
		// { agent_response_event: { agent_response: "..." } }
		var ev struct {
			ARE struct {
				Text string `json:"agent_response"`
			} `json:"agent_response_event"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.ARE.Text != "" {
			emit(s, session.Event{Type: "transcript", Role: "agent", Text: ev.ARE.Text, Final: true})
			s.AppendTranscript("agent", ev.ARE.Text)
		}
	case "audio":
		// { audio_event: { audio_base_64: "...", event_id } }
		var ev struct {
			AE struct {
				AudioB64 string `json:"audio_base_64"`
			} `json:"audio_event"`
		}
		if err := json.Unmarshal(raw, &ev); err == nil && ev.AE.AudioB64 != "" {
			if pcm16, err := base64.StdEncoding.DecodeString(ev.AE.AudioB64); err == nil {
				// Downsample 16k → 8k para AudioSocket.
				pcm8 := audio.DownsampleSlin16kTo8k(pcm16)
				select {
				case s.AudioOut <- pcm8:
					emit(s, session.Event{Type: "status", State: "speaking"})
				case <-s.Context().Done():
					return
				}
			}
		}
	case "interruption":
		// Usuario interrumpió al agente. Barge-in: vaciar el TTS pendiente.
		emit(s, session.Event{Type: "status", State: "listening"})
		flushAudioOut(s)
	case "client_tool_call":
		// { client_tool_call: { tool_name, tool_call_id, parameters } }
		e.handleToolCall(ctx, conn, raw, s)
	case "agent_response_correction":
		// El modelo corrige una respuesta — útil para logging, no afecta audio.
	case "internal_tentative_agent_response", "agent_response_complete":
		// Eventos internos del agente. No-op.
	case "contextual_update":
		// Updates de contexto (variables dinámicas). No-op por ahora.
	case "error":
		var ev struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &ev)
		e.logger.Warn("elevenlabs error event", "session", s.ID, "msg", ev.Message)
		emit(s, session.Event{Type: "error", Message: ev.Message})
	}
}

// handleToolCall procesa client_tool_call de ElevenLabs y responde con
// client_tool_result. Schema:
//
//	{ type: "client_tool_call",
//	  client_tool_call: { tool_name, tool_call_id, parameters: {...} } }
//
// Respuesta:
//
//	{ type: "client_tool_result",
//	  tool_call_id, result: "...", is_error: false }
func (e *ElevenLabs) handleToolCall(ctx context.Context, conn *websocket.Conn, raw []byte, s *session.Session) {
	var ev struct {
		Call struct {
			Name       string         `json:"tool_name"`
			ID         string         `json:"tool_call_id"`
			Parameters map[string]any `json:"parameters"`
		} `json:"client_tool_call"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		e.logger.Warn("elevenlabs tool call parse", "error", err)
		return
	}
	if ev.Call.Name == "" || ev.Call.ID == "" {
		return
	}
	content, ok := s.InvokeTool(ctx, ev.Call.Name, ev.Call.Parameters)
	resp := map[string]any{
		"type":         "client_tool_result",
		"tool_call_id": ev.Call.ID,
		"result":       content,
		"is_error":     !ok,
	}
	if err := writeJSON(ctx, conn, resp); err != nil {
		e.logger.Warn("elevenlabs tool result write", "tool", ev.Call.Name, "error", err)
	}
}

// elevenlabsLang normaliza el locale ("es-ES" → "es") a ISO 639-1.
// Mismo enfoque que deepgramLang para mantener consistencia entre
// providers.
func elevenlabsLang(locale string) string {
	if i := indexAny(locale, "-_"); i > 0 {
		return locale[:i]
	}
	return locale
}

func indexAny(s, chars string) int {
	for i, c := range s {
		for _, ch := range chars {
			if c == ch {
				return i
			}
		}
	}
	return -1
}

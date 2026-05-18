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

// AssemblyAI talks to the Voice Agent WebSocket API end-to-end:
//
//	wss://agents.assemblyai.com/v1/ws
//
// AssemblyAI hosts ASR + LLM + TTS internally. La API requiere PCM16
// 24 kHz mono en ambos sentidos (input.audio y reply.audio), pero
// nosotros venimos de AudioSocket a 8 kHz. Hacemos resampling 8↔24 con
// audio.UpsampleSlin8kTo24k / audio.DownsampleSlin24kTo8k antes/después
// del WS, lo que evita el bug histórico de "el bot suena 3× rápido".
//
// Audio I/O: base64-encoded PCM16 embebido en JSON (no binario crudo
// como Deepgram), por las restricciones del endpoint de AssemblyAI.
//
// Spec:
//
//	https://www.assemblyai.com/docs/voice-agents/voice-agent-api
//	https://www.assemblyai.com/docs/voice-agents/voice-agent-api/session-configuration
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

	// session.update — config completa de la sesión. Schema oficial:
	//   { type:"session.update", session:{ system_prompt, greeting,
	//     tools, input:{ format, keyterms, turn_detection },
	//     output:{ voice, format } } }
	settings := map[string]any{
		"type": "session.update",
		"session": map[string]any{
			"system_prompt": SystemPrompt(s.Config),
			"greeting":      greeting,
			"input": map[string]any{
				// Default es audio/pcm 24 kHz; lo declaramos explícito.
				"format": map[string]any{"encoding": "pcm_s16le"},
				// turn_detection: defaults documentados son vad_threshold 0.5,
				// min_silence 1000 ms, max_silence 3000 ms. min_silence 1000
				// es demasiado lento para conversación natural — bajamos a
				// 600 ms (alineado con OpenAI Realtime y Deepgram).
				"turn_detection": map[string]any{
					"vad_threshold":      0.5,
					"min_silence":        600,
					"max_silence":        2500,
					"interrupt_response": true,
				},
			},
			"output": map[string]any{
				"voice":  voice,
				"format": map[string]any{"encoding": "pcm_s16le"},
			},
			// Tools: idéntica forma que OpenAI Realtime. Inyectamos
			// también la no-op wait_for_user para silencios/ruido.
			"tools": buildAssemblyAITools(s.Config.Tools),
		},
	}
	if err := writeJSON(ctx, conn, settings); err != nil {
		return err
	}

	// Esperamos session.ready antes de pumpear audio — la doc dice
	// expresamente: "Audio streaming begins only after session.ready".
	// Sin esto los primeros frames se pierden y el agente tarda en
	// arrancar.
	if err := a.waitSessionReady(ctx, conn, 10*time.Second); err != nil {
		emit(s, session.Event{Type: "error", Message: "assemblyai_ready: " + err.Error()})
		return fmt.Errorf("assemblyai session.ready: %w", err)
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// Goroutine: pump caller audio (slin 8k de AudioSocket) → upsample
	// a 24 kHz → base64 JSON. Necesitamos los dos pasos: la API rechaza
	// audio que no esté a 24 kHz, y AudioSocket solo sabe entregar 8.
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
				up := audio.UpsampleSlin8kTo24k(chunk)
				_ = writeJSON(ctx, conn, map[string]any{
					"type":  "input.audio",
					"audio": base64.StdEncoding.EncodeToString(up),
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
		a.handleEvent(ctx, conn, raw, s)
	}
}

// waitSessionReady drena el WS hasta encontrar session.ready, abortando
// si llega session.error o se agota el timeout.
func (a *AssemblyAI) waitSessionReady(ctx context.Context, conn *websocket.Conn, timeout time.Duration) error {
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
		case "session.ready":
			return nil
		case "session.error":
			return fmt.Errorf("assemblyai error: %s", probe.Message)
		}
	}
}

func (a *AssemblyAI) handleEvent(ctx context.Context, conn *websocket.Conn, raw []byte, s *session.Session) {
	var probe struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return
	}
	switch probe.Type {
	case "session.ready":
		// Ya manejado en waitSessionReady; aquí no-op.
	case "input.speech.started":
		emit(s, session.Event{Type: "status", State: "listening"})
		// Barge-in: el usuario interrumpe al agente. Vaciamos AudioOut
		// para que AudioSocket deje de mandar TTS al caller. Mismo patrón
		// que OpenAI Realtime y Deepgram.
		flushAudioOut(s)
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
			Text string `json:"text"`
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
			if pcm24, err := base64.StdEncoding.DecodeString(ev.Audio); err == nil {
				// Downsample 24k → 8k para que AudioSocket pueda enviarlo
				// al trunk SIP sin transcoding adicional.
				pcm8 := audio.DownsampleSlin24kTo8k(pcm24)
				select {
				case s.AudioOut <- pcm8:
				case <-s.Context().Done():
					return
				}
			}
		}
	case "reply.done":
		emit(s, session.Event{Type: "status", State: "listening"})
	case "tool.call":
		// Schema: { type:"tool.call", id, name, arguments }
		a.handleToolCall(ctx, conn, raw, s)
	case "session.error":
		var ev struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &ev)
		a.logger.Warn("assemblyai agent error", "msg", ev.Message)
		emit(s, session.Event{Type: "error", Message: ev.Message})
	}
}

// handleToolCall procesa un tool.call de AssemblyAI: extrae name/args,
// invoca al backend vía session.InvokeTool y manda tool.result. Igual
// que en los otros providers, wait_for_user es un no-op (solo cerramos
// el turno sin pedir respuesta nueva).
func (a *AssemblyAI) handleToolCall(ctx context.Context, conn *websocket.Conn, raw []byte, s *session.Session) {
	var ev struct {
		ID        string `json:"id"`
		Name      string `json:"name"`
		Arguments any    `json:"arguments"`
	}
	if err := json.Unmarshal(raw, &ev); err != nil {
		a.logger.Warn("assemblyai tool.call parse", "error", err)
		return
	}
	if ev.Name == "" || ev.ID == "" {
		return
	}

	// AssemblyAI manda arguments como objeto JSON, no como string.
	args := map[string]any{}
	switch v := ev.Arguments.(type) {
	case map[string]any:
		args = v
	case string:
		if v != "" {
			_ = json.Unmarshal([]byte(v), &args)
		}
	}

	if ev.Name == "wait_for_user" {
		_ = writeJSON(ctx, conn, map[string]any{
			"type":   "tool.result",
			"id":     ev.ID,
			"result": map[string]any{"acknowledged": true},
		})
		return
	}

	content, ok := s.InvokeTool(ctx, ev.Name, args)
	if !ok {
		a.logger.Warn("assemblyai tool invoke failed", "tool", ev.Name)
	}
	_ = writeJSON(ctx, conn, map[string]any{
		"type":   "tool.result",
		"id":     ev.ID,
		"result": content,
	})
}

// buildAssemblyAITools convierte las tools de la sesión al schema que
// espera AssemblyAI en session.update. Inyectamos también wait_for_user
// como no-op (la guía oficial recomienda la equivalente — mismo
// principio que en OpenAI Realtime).
func buildAssemblyAITools(sessTools []session.Tool) []map[string]any {
	tools := []map[string]any{{
		"name":        "wait_for_user",
		"description": "Llama a esta tool cuando el audio recibido sea silencio, ruido de fondo, música de espera, o una conversación entre el caller y un tercero que no se dirige a ti. No respondas conversacionalmente cuando uses esta tool.",
		"parameters": map[string]any{
			"type":       "object",
			"properties": map[string]any{},
		},
	}}
	for _, t := range sessTools {
		tools = append(tools, map[string]any{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		})
	}
	return tools
}

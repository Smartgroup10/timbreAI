package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/audio"
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
	// NO añadir "OpenAI-Beta: realtime=v1" — la doc oficial de GA dice
	// literalmente "Remove the OpenAI-Beta: realtime=v1 header when
	// calling the GA interface". Con el header, OpenAI sirve el endpoint
	// preview que no entiende los Settings nuevos y la sesión nunca
	// arranca (el bot no habla). Encontrado tras debugging en producción.

	conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		emit(s, session.Event{Type: "error", Message: "openai_dial: " + err.Error()})
		return fmt.Errorf("openai realtime dial: %w", err)
	}
	defer conn.Close(websocket.StatusNormalClosure, "session_end")
	// OpenAI Realtime manda audio deltas grandes (especialmente al greeting).
	// coder/websocket por defecto limita a 32 KB y dispara "read limited at
	// 32769 bytes" tirando el provider. 10 MB sobra para cualquier frame.
	conn.SetReadLimit(10 * 1024 * 1024)

	voice := pick(s.Config.Voice, pick(s.Config.Credentials.OpenAIRealtimeVoice, o.cfg.Voice))

	// Codec: g711_ulaw (8 kHz μ-law). OJO: OpenAI Realtime "pcm16" implica
	// 24 kHz signed linear. Si le mandas slin a 8 kHz (lo que llega de
	// AudioSocket) diciéndole que es pcm16, OpenAI lo interpreta a 24 kHz y
	// se acelera 3x → el LLM no entiende nada. Usamos g711_ulaw para que
	// ambos lados hablen en 8 kHz y solo convertimos slin↔ulaw aquí.
	// Patrón copiado de Smartgroup10/SmartSIP.
	sessionCfg := map[string]any{
		"modalities":          []string{"audio", "text"},
		"instructions":        SystemPrompt(s.Config),
		"voice":               voice,
		"input_audio_format":  "g711_ulaw",
		"output_audio_format": "g711_ulaw",
		"turn_detection": map[string]any{
			"type":                "server_vad",
			"threshold":           0.5,
			"prefix_padding_ms":   300,
			"silence_duration_ms": 600,
		},
		"input_audio_transcription": map[string]any{"model": "whisper-1"},
		// NOTA: NO añadir "reasoning.effort" aquí. La guía de prompting
		// lo menciona como concepto, pero el campo "reasoning" es propio
		// de la Responses API, no de Realtime session.update. Cuando se
		// envía, OpenAI lo rechaza silenciosamente y la sesión queda en
		// estado inválido — el bot no habla. Eliminado tras encontrar
		// audio roto en producción (commit anterior lo había añadido).
	}
	// Tools: OpenAI Realtime acepta [{type:"function", name, description, parameters}].
	// Sin esto el LLM nunca emitirá function_call.
	//
	// NOTA sobre wait_for_user: la guía oficial la recomienda como no-op
	// para silencios mid-conversación. La habíamos inyectado pero
	// interfería con el greeting inicial: al hacer response.create el
	// modelo a veces decidía llamar wait_for_user (porque "no hay audio
	// del user todavía") en vez de saludar → bot mudo. La reactivaremos
	// cuando podamos forzar tool_choice por turno (greeting con none,
	// resto con auto).
	if len(s.Config.Tools) > 0 {
		tools := make([]map[string]any, 0, len(s.Config.Tools))
		for _, t := range s.Config.Tools {
			tools = append(tools, map[string]any{
				"type":        "function",
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
		sessionCfg["tools"] = tools
		sessionCfg["tool_choice"] = "auto"
	}
	initMsg := map[string]any{
		"type":    "session.update",
		"session": sessionCfg,
	}
	if err := writeJSON(ctx, conn, initMsg); err != nil {
		return err
	}

	// Disparar el greeting: response.create hace que el modelo hable PRIMERO
	// (usando las instructions del system prompt). Sin esto el agente espera
	// a que el caller hable antes de saludar, y la llamada se siente muerta.
	// Patrón copiado de Smartgroup10/SmartSIP que sí funciona en prod.
	if err := writeJSON(ctx, conn, map[string]any{"type": "response.create"}); err != nil {
		o.logger.Warn("openai response.create", "session", s.ID, "error", err)
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
		o.handleEvent(ctx, conn, msg, s)
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
			// AudioSocket nos da slin 8 kHz (320 B/20 ms). OpenAI espera
			// g711_ulaw 8 kHz (160 B/20 ms). Convertimos.
			ulaw := audio.SlinToUlaw(chunk)
			_ = writeJSON(ctx, conn, map[string]any{
				"type":  "input_audio_buffer.append",
				"audio": base64.StdEncoding.EncodeToString(ulaw),
			})
		}
	}
}

func (o *OpenAIRealtime) handleEvent(ctx context.Context, conn *websocket.Conn, msg map[string]any, s *session.Session) {
	t, _ := msg["type"].(string)
	switch t {
	case "response.audio.delta":
		// base64 g711_ulaw 8 kHz mono. AudioSocket espera slin (8 kHz signed
		// linear 16-bit), así que convertimos antes de pushear a AudioOut.
		if data, _ := msg["delta"].(string); data != "" {
			if ulaw, err := base64.StdEncoding.DecodeString(data); err == nil {
				pcm := audio.UlawToSlin(ulaw)
				select {
				case s.AudioOut <- pcm:
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
		// Barge-in equivalente al de Deepgram: vaciar AudioOut para cortar
		// el TTS pendiente que aún no haya salido al caller.
		flushAudioOut(s)
	case "input_audio_buffer.speech_stopped":
		emit(s, session.Event{Type: "status", State: "thinking"})
	case "response.created":
		emit(s, session.Event{Type: "status", State: "speaking"})
	case "response.done":
		emit(s, session.Event{Type: "status", State: "listening"})
		// Cuando la response.done contiene output items de tipo "function_call",
		// el LLM nos pide que ejecutemos una tool. Mismo evento que en el SDK
		// estándar de OpenAI — la API Realtime cierra el turno y nos manda el
		// arguments completo en el item.
		o.dispatchFunctionCalls(ctx, conn, s, msg)
	case "error":
		emit(s, session.Event{Type: "error", Message: jsonString(msg, "error", "message")})
	}
}

// dispatchFunctionCalls extrae los items de tipo "function_call" del payload
// de response.done, ejecuta cada uno via session.InvokeTool y envía a OpenAI
// el resultado como conversation.item.create + response.create para que el
// LLM continúe el turno con el output disponible.
func (o *OpenAIRealtime) dispatchFunctionCalls(ctx context.Context, conn *websocket.Conn, s *session.Session, msg map[string]any) {
	response, _ := msg["response"].(map[string]any)
	if response == nil {
		return
	}
	output, _ := response["output"].([]any)
	if len(output) == 0 {
		return
	}
	dispatched := false
	for _, raw := range output {
		item, _ := raw.(map[string]any)
		if item == nil {
			continue
		}
		if t, _ := item["type"].(string); t != "function_call" {
			continue
		}
		name, _ := item["name"].(string)
		callID, _ := item["call_id"].(string)
		argsStr, _ := item["arguments"].(string)
		if name == "" || callID == "" {
			continue
		}

		args := map[string]any{}
		if argsStr != "" {
			_ = json.Unmarshal([]byte(argsStr), &args)
		}
		// Síncrono pero corto — InvokeTool tiene timeout en el webhook client.
		content, ok := s.InvokeTool(ctx, name, args)
		if !ok {
			o.logger.Warn("openai tool invoke failed", "tool", name)
		}
		// Empujamos el resultado como conversation.item function_call_output.
		if err := writeJSON(ctx, conn, map[string]any{
			"type": "conversation.item.create",
			"item": map[string]any{
				"type":    "function_call_output",
				"call_id": callID,
				"output":  content,
			},
		}); err != nil {
			o.logger.Warn("openai conversation.item.create", "error", err)
			continue
		}
		dispatched = true
	}
	if dispatched {
		// Pedimos al LLM que produzca la siguiente respuesta consumiendo los
		// outputs. Sin response.create el modelo se queda esperando.
		_ = writeJSON(ctx, conn, map[string]any{"type": "response.create"})
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

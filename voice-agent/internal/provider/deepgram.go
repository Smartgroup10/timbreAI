package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"

	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// deepgramLang normaliza el locale del bot ("es-ES", "en-US", "PT-br")
// al código corto que espera la API de Deepgram Voice Agent (ISO 639-1
// en minúsculas: "es", "en", "pt"). Vacío → "en" como en smartsip.
//
// SIN este campo el listen (ASR) usa el default de Deepgram (inglés),
// así que aunque el TTS sea aura-2-celeste-es el bot no entiende lo que
// dice el usuario en español. Es el bug "el bot no me escucha" en
// llamadas en castellano.
func deepgramLang(locale string) string {
	locale = strings.TrimSpace(locale)
	if locale == "" {
		return "en"
	}
	if i := strings.IndexAny(locale, "-_"); i > 0 {
		locale = locale[:i]
	}
	return strings.ToLower(locale)
}

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

// Sample rate del audio que mandamos a Deepgram. AudioSocket usa slin nativo
// a 8 kHz (mismo que el codec del trunk SIP), así que evitamos cualquier
// resampling. Si vuelves a External Media + slin16 cambia a 16000.
const deepgramSampleRate = 8000

func (d *Deepgram) Run(ctx context.Context, s *session.Session) error {
	apiKey := pick(s.Config.Credentials.DeepgramAPIKey, d.cfg.APIKey)
	listenModel := pick(s.Config.Credentials.DeepgramListenModel, d.cfg.ListenModel)
	thinkProvider := pick(s.Config.Credentials.DeepgramThinkProvider, d.cfg.ThinkProvider)
	thinkModel := pick(s.Config.Credentials.DeepgramThinkModel, d.cfg.ThinkModel)
	speakModel := pick(s.Config.Credentials.DeepgramSpeakModel, d.cfg.SpeakModel)
	greeting := pick(s.Config.Credentials.DeepgramGreeting, d.cfg.Greeting)
	// Para think providers externos (open_ai, anthropic, ...) Deepgram necesita
	// la API key del LLM. Aquí cogemos la del propio tenant — si el cliente
	// ha configurado OpenAI key en /portal/settings, la reutilizamos para que
	// Deepgram pueda llamar a OpenAI en su nombre.
	openaiKey := pick(s.Config.Credentials.OpenAIAPIKey, "")

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
	// 32 KB default es muy poco — audio TTS de Aura puede llegar en frames
	// más grandes y mata el provider con "read limited".
	conn.SetReadLimit(10 * 1024 * 1024)

	// Deepgram Voice Agent handshake: el servidor manda "Welcome" primero,
	// luego nosotros mandamos "Settings", luego él contesta "SettingsApplied".
	// Sin esto la sesión queda en estado inválido y se cierra silenciosamente
	// (referencia: Smartgroup10/SmartSIP/services/go-service/.../deepgram_agent.go).
	if err := waitDeepgramEvent(ctx, conn, "Welcome", 10*time.Second); err != nil {
		emit(s, session.Event{Type: "error", Message: "deepgram_welcome: " + err.Error()})
		return fmt.Errorf("deepgram welcome: %w", err)
	}

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
			// `agent.language` top-level configura el ASR. La doc dice que
			// está deprecado a favor de `listen.provider.language` /
			// `speak.provider.language`, pero la forma top-level sigue
			// funcionando y es la que tenemos verificada en producción.
			// No la quitamos hasta que tengamos pruebas E2E con la nueva.
			"language": deepgramLang(s.Config.Language),
			"listen": map[string]any{
				"provider": map[string]any{
					"type":  "deepgram",
					"model": listenModel,
					// NOTA: smart_format y reasoning_mode están documentados
					// pero su soporte depende del modelo + del think provider.
					// Cuando los añadimos en bulk causaron que Settings
					// fuera rechazado silenciosamente (bot sin hablar).
					// Re-añadir uno por uno tras validación E2E.
				},
			},
			"think": d.buildThinkSection(thinkProvider, thinkModel, openaiKey, SystemPrompt(s.Config), s.Config.Tools),
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
	if err := waitDeepgramEvent(ctx, conn, "SettingsApplied", 10*time.Second); err != nil {
		emit(s, session.Event{Type: "error", Message: "deepgram_settings_applied: " + err.Error()})
		return fmt.Errorf("deepgram settings applied: %w", err)
	}
	emit(s, session.Event{Type: "status", State: "ready"})

	// KeepAlive: Deepgram cierra la conexión si pasan ~10s sin tráfico binario
	// O de control. Mientras el caller esté en silencio (no manda audio) el
	// WS se cae. Mandamos {"type":"KeepAlive"} cada 5s.
	keepAliveCtx, keepAliveCancel := context.WithCancel(ctx)
	defer keepAliveCancel()
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-keepAliveCtx.Done():
				return
			case <-t.C:
				_ = writeJSON(keepAliveCtx, conn, map[string]any{"type": "KeepAlive"})
			}
		}
	}()

	// Goroutine: pump caller audio (binary PCM16) to Deepgram.
	// Contadores para diagnosticar "Deepgram dice CLIENT_MESSAGE_TIMEOUT" —
	// si chunks=0 sabemos que el problema está aguas arriba (RTP listener
	// o Asterisk no manda); si chunks>0 pero Deepgram se queja, el problema
	// es de formato/sample-rate.
	go func() {
		var chunks, bytesTotal uint64
		var writeErrs uint64
		statTick := time.NewTicker(5 * time.Second)
		defer statTick.Stop()
		for {
			select {
			case <-ctx.Done():
				d.logger.Info("deepgram pump exit", "session", s.ID,
					"chunks_sent", chunks, "bytes", bytesTotal, "write_errors", writeErrs)
				return
			case <-statTick.C:
				if chunks > 0 || writeErrs > 0 {
					d.logger.Info("deepgram pump stats", "session", s.ID,
						"chunks_sent", chunks, "bytes", bytesTotal, "write_errors", writeErrs)
				}
			case chunk, ok := <-s.AudioIn:
				if !ok {
					return
				}
				if err := conn.Write(ctx, websocket.MessageBinary, chunk); err != nil {
					writeErrs++
				} else {
					chunks++
					bytesTotal += uint64(len(chunk))
				}
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
			d.handleEvent(ctx, conn, raw, s)
		}
	}
}

// waitDeepgramEvent lee del WS hasta encontrar un evento JSON con el `type`
// indicado. Ignora frames binarios (no aplica durante handshake) y otros
// eventos de texto. Aborta si llega un Error o se agota el timeout.
func waitDeepgramEvent(ctx context.Context, conn *websocket.Conn, expected string, timeout time.Duration) error {
	deadline, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for {
		msgType, raw, err := conn.Read(deadline)
		if err != nil {
			return err
		}
		if msgType != websocket.MessageText {
			continue
		}
		var probe struct {
			Type        string `json:"type"`
			Description string `json:"description"`
		}
		if err := json.Unmarshal(raw, &probe); err != nil {
			continue
		}
		switch probe.Type {
		case expected:
			return nil
		case "Error", "ErrorResponse":
			return fmt.Errorf("deepgram error: %s", probe.Description)
		}
		// Cualquier otro evento (Warning, etc.) lo ignoramos hasta el esperado.
	}
}

// buildThinkSection arma el bloque "think" del Settings de Deepgram. Para los
// providers externos open_ai/anthropic, Deepgram NECESITA que le pasemos las
// credenciales del LLM en `endpoint.headers.authorization` — si no, la WS se
// queda abierta, recibe audio, pero nunca responde (el LLM call falla en
// silencio dentro de Deepgram).
//
// Si el operador no ha configurado la key del LLM externo, NO usamos endpoint
// y dejamos que Deepgram intente con sus credenciales por defecto (puede
// funcionar si el proyecto Deepgram tiene la integración pre-configurada).
//
// Tools: si hay tools definidas en la sesión las añadimos como functions
// en el think — Deepgram las pasa al LLM y nos devuelve FunctionCallRequest
// cuando el LLM decide invocar alguna.
func (d *Deepgram) buildThinkSection(provider, model, externalKey, prompt string, tools []session.Tool) map[string]any {
	think := map[string]any{
		"provider": map[string]any{
			"type":  provider,
			"model": model,
			// NOTA: reasoning_mode aparece en la doc pero parece no estar
			// soportado por todos los modelos (gpt-4o-mini lo rechaza
			// silenciosamente y la Settings falla). Reactivar selectivamente
			// cuando tengamos un E2E test que verifique la respuesta.
		},
		"prompt": prompt,
	}
	if len(tools) > 0 {
		fns := make([]map[string]any, 0, len(tools))
		for _, t := range tools {
			fns = append(fns, map[string]any{
				"name":        t.Name,
				"description": t.Description,
				"parameters":  t.Parameters,
			})
		}
		think["functions"] = fns
	}
	if externalKey == "" {
		return think
	}
	// Mapa provider → endpoint URL canónico.
	endpointURL := ""
	switch provider {
	case "open_ai":
		endpointURL = "https://api.openai.com/v1/chat/completions"
	case "anthropic":
		endpointURL = "https://api.anthropic.com/v1/messages"
	}
	if endpointURL == "" {
		return think
	}
	think["endpoint"] = map[string]any{
		"url": endpointURL,
		"headers": map[string]any{
			"authorization": "Bearer " + externalKey,
		},
	}
	return think
}

func (d *Deepgram) handleEvent(ctx context.Context, conn *websocket.Conn, raw []byte, s *session.Session) {
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
		// Barge-in: el usuario interrumpe al agente. Vaciamos el canal de
		// audio TTS pendiente para que el AudioSocket deje de mandar frames
		// al caller en cuanto detecta voz. Sin esto, si el agente está en
		// medio de una frase larga, el caller la sigue oyendo varios cientos
		// de ms después de empezar a hablar — UX de "robot que no escucha".
		flushAudioOut(s)
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
	case "FunctionCallRequest":
		// Schema: { type:"FunctionCallRequest", functions:[{id,name,arguments,client_side}] }
		var fcr struct {
			Functions []struct {
				ID         string `json:"id"`
				Name       string `json:"name"`
				Arguments  string `json:"arguments"` // JSON-encoded string
				ClientSide bool   `json:"client_side"`
			} `json:"functions"`
		}
		if err := json.Unmarshal(raw, &fcr); err != nil {
			d.logger.Warn("deepgram FunctionCallRequest parse", "error", err)
			return
		}
		for _, fc := range fcr.Functions {
			if !fc.ClientSide {
				continue
			}
			d.handleFunctionCall(ctx, conn, s, fc.ID, fc.Name, fc.Arguments)
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

// handleFunctionCall delega la invocación al backend via la session hook
// y devuelve el resultado al Voice Agent como FunctionCallResponse. Lo
// hacemos en goroutine para no bloquear el read loop principal — si la
// tool tarda 5s el audio sigue fluyendo.
func (d *Deepgram) handleFunctionCall(ctx context.Context, conn *websocket.Conn, s *session.Session, callID, name, argsJSON string) {
	go func() {
		// El payload "arguments" viene como string JSON-encoded.
		args := map[string]any{}
		if argsJSON != "" {
			if err := json.Unmarshal([]byte(argsJSON), &args); err != nil {
				d.logger.Warn("deepgram function args parse", "tool", name, "error", err)
			}
		}
		content, ok := s.InvokeTool(ctx, name, args)
		if !ok {
			d.logger.Warn("deepgram tool invoke failed", "tool", name)
		}
		// Respuesta al provider — el LLM la verá como output de la function.
		resp := map[string]any{
			"type":    "FunctionCallResponse",
			"id":      callID,
			"name":    name,
			"content": content,
		}
		if err := writeJSON(ctx, conn, resp); err != nil {
			d.logger.Warn("deepgram function response", "tool", name, "error", err)
		}
	}()
}

package provider

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	"timbre/voice-agent/internal/config"
	"timbre/voice-agent/internal/session"
)

// Provider runs a single live conversation. Implementations bridge the session's audio channels
// with whatever realtime service they wrap (OpenAI Realtime, Deepgram, AssemblyAI, ...).
type Provider interface {
	Name() string
	Run(ctx context.Context, s *session.Session) error
}

var ErrNotConfigured = errors.New("provider_not_configured")

// pick returns sessionVal if non-empty, else fallback. Used to layer per-tenant credentials over
// the voice-agent's env defaults.
func pick(sessionVal, fallback string) string {
	if sessionVal != "" {
		return sessionVal
	}
	return fallback
}

// SystemPrompt construye el bloque de instrucciones para el LLM con la
// estructura recomendada por la guía oficial de OpenAI Realtime
// (https://developers.openai.com/api/docs/guides/realtime-models-prompting).
//
// La guía insiste en secciones cortas y etiquetadas: el modelo localiza
// la instrucción relevante por título de sección. Por orden:
//
//	# Role & Objective    — quién es y para qué llama
//	# Personality & Tone  — cómo sonar (conversacional, no robótico)
//	# Language            — idioma del usuario; no cambiar por acento
//	# Unclear Audio       — qué hacer si no se oye bien
//	# Entity Capture      — cómo confirmar números/emails dígito a dígito
//	# Tools               — cuándo invocar function calls
//	# Guardrails          — reglas del operador (objective custom)
//	# Escalation          — cuándo proponer un humano
//
// Los providers (OpenAI Realtime, Deepgram-LLM, AssemblyAI-LLM) reciben
// este prompt verbatim. Deepgram y AssemblyAI tienen LLMs distintos pero
// el estilo Markdown les sienta bien también.
func SystemPrompt(cfg session.Config) string {
	var b strings.Builder

	// ─── Role & Objective ────────────────────────────────────────────
	b.WriteString("# Role & Objective\n")
	b.WriteString("Eres un asistente de voz IA llamando en nombre de una agencia inmobiliaria. ")
	if cfg.LeadName != "" {
		b.WriteString("Estás hablando con ")
		b.WriteString(cfg.LeadName)
		b.WriteString(". ")
	}
	if cfg.Objective != "" {
		b.WriteString("Objetivo de la llamada: ")
		b.WriteString(strings.TrimSpace(cfg.Objective))
		b.WriteString("\n\n")
	} else {
		b.WriteString("Calificar al lead y, si hay interés, agendar una visita.\n\n")
	}

	// ─── Personality & Tone ──────────────────────────────────────────
	b.WriteString("# Personality & Tone\n")
	b.WriteString("- Profesional pero cálido. No robótico.\n")
	b.WriteString("- Conciso: respuestas de 1–2 frases salvo que el lead pida detalle.\n")
	b.WriteString("- Identifícate como asistente IA si te lo preguntan directamente.\n")
	b.WriteString("- No inventes información (precios, disponibilidad). Si no la sabes, dilo.\n\n")

	// ─── Language ────────────────────────────────────────────────────
	if cfg.Language != "" {
		b.WriteString("# Language\n")
		b.WriteString("Responde siempre en ")
		b.WriteString(cfg.Language)
		b.WriteString(". No cambies de idioma por el acento del usuario; cambia solo si el usuario te pide explícitamente otro idioma o usa frases completas en otro idioma.\n\n")
	}

	// ─── Unclear Audio ───────────────────────────────────────────────
	// Bloque exacto que recomienda la guía: evita que el bot inventa
	// respuestas cuando hay ruido, silencio o solapamiento.
	b.WriteString("# Unclear Audio\n")
	b.WriteString("- Responde solo a audio o texto claro.\n")
	b.WriteString("- Si el audio del usuario no es claro, pide aclaración con una frase corta (\"¿Podrías repetirlo, por favor?\").\n")
	b.WriteString("- No repitas la misma aclaración dos veces seguidas.\n")
	b.WriteString("- Si solo hay silencio o ruido de fondo, no respondas — llama a la tool `wait_for_user` si está disponible.\n\n")

	// ─── Entity Capture ──────────────────────────────────────────────
	// Patrón recomendado para teléfonos, emails y otros identificadores.
	b.WriteString("# Entity Capture\n")
	b.WriteString("- Recoge un dato cada vez. No pidas varios datos en la misma pregunta.\n")
	b.WriteString("- Para números (teléfonos, códigos): repite el valor capturado dígito a dígito y espera confirmación antes de seguir.\n")
	b.WriteString("  Ejemplo: \"He oído seis-seis-seis-uno-dos-tres-cuatro-cinco-seis-siete. ¿Es correcto?\"\n")
	b.WriteString("- Para emails: pide deletreo carácter a carácter y confirma de vuelta.\n")
	b.WriteString("- Si el dato suena ambiguo (\"ciento diecinueve\" vs \"uno uno nueve\"), pide repetir dígito a dígito.\n\n")

	// ─── Tools ───────────────────────────────────────────────────────
	if len(cfg.Tools) > 0 {
		b.WriteString("# Tools\n")
		b.WriteString("- Usa solo las tools listadas en la sesión actual. No inventes nombres ni simules llamadas.\n")
		b.WriteString("- Para acciones de solo lectura (búsqueda, consulta) con la intención clara, llama directamente.\n")
		b.WriteString("- Para acciones que escriben o tienen efecto externo (agendar, cancelar, enviar): confirma con el usuario antes de llamar.\n")
		b.WriteString("- Si una tool falla, explica brevemente qué pasó y ofrece un siguiente paso.\n\n")
	}

	// ─── Guardrails (las reglas custom del operador del bot) ─────────
	if len(cfg.Guardrails) > 0 {
		b.WriteString("# Guardrails\n")
		for _, g := range cfg.Guardrails {
			g = strings.TrimSpace(g)
			if g == "" {
				continue
			}
			b.WriteString("- ")
			b.WriteString(g)
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// ─── Escalation ──────────────────────────────────────────────────
	b.WriteString("# Escalation\n")
	b.WriteString("- Si el lead se enfada, pide hablar con una persona, o haces una pregunta que no sabes resolver: ofrece pasarlo a un humano y registra el motivo.\n")
	b.WriteString("- Si el lead dice claramente \"no me llaméis más\", confirma y registra el opt-out.\n")

	return b.String()
}

// Registry maps a provider name to its concrete implementation. Built once at startup.
type Registry struct {
	providers map[string]Provider
	logger    *slog.Logger
}

func NewRegistry(cfg config.Config, logger *slog.Logger) *Registry {
	r := &Registry{providers: map[string]Provider{}, logger: logger}
	r.register(&Echo{logger: logger})
	// All providers are always registered — they decide at Run() time whether they have enough
	// credentials (per-tenant override OR env default) to actually start a session.
	r.register(NewOpenAIRealtime(cfg.OpenAI, logger))
	r.register(NewDeepgram(cfg.Deepgram, logger))
	r.register(NewAssemblyAI(cfg.AssemblyAI, logger))
	return r
}

func (r *Registry) register(p Provider) {
	r.providers[p.Name()] = p
	r.logger.Info("provider registered", "name", p.Name())
}

func (r *Registry) Get(name string) (Provider, bool) {
	if name == "" {
		name = "echo"
	}
	p, ok := r.providers[name]
	return p, ok
}

func (r *Registry) Names() []string {
	out := make([]string, 0, len(r.providers))
	for k := range r.providers {
		out = append(out, k)
	}
	return out
}

// flushAudioOut drena el canal de audio TTS pendiente. Lo usamos para
// implementar barge-in: cuando el provider detecta que el usuario empezó
// a hablar, descartamos cualquier frame de audio del agente que aún no
// haya salido al caller, de forma que el sender de AudioSocket deje de
// transmitir TTS "viejo" casi al instante.
//
// Non-blocking: si no hay frames, sale al toque.
func flushAudioOut(s *session.Session) {
	for {
		select {
		case <-s.AudioOut:
			// descartado
		default:
			return
		}
	}
}

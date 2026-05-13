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

// SystemPrompt builds an instruction block from a bot's objective and guardrails.
// All three downstream pipelines (OpenAI, Deepgram-LLM, AssemblyAI-LLM) take this verbatim.
func SystemPrompt(cfg session.Config) string {
	var b strings.Builder
	b.WriteString("Eres un asistente de voz IA. ")
	if cfg.Objective != "" {
		b.WriteString("Objetivo de la llamada: ")
		b.WriteString(cfg.Objective)
		b.WriteString(". ")
	}
	if len(cfg.Guardrails) > 0 {
		b.WriteString("Reglas estrictas:\n")
		for _, g := range cfg.Guardrails {
			b.WriteString(" - ")
			b.WriteString(g)
			b.WriteString("\n")
		}
	}
	if cfg.Language != "" {
		b.WriteString("Responde siempre en ")
		b.WriteString(cfg.Language)
		b.WriteString(". ")
	}
	b.WriteString("Sé conciso, natural y conversacional. Si no sabes algo, dilo y ofrece pasar a un humano.")
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

package config

import (
	"errors"
	"os"
	"strings"
	"time"
)

type Config struct {
	Port            string
	BackendURL      string
	BackendAuthKey  string
	AllowedOrigins  []string
	SessionTTL      time.Duration

	RTP         RTPConfig
	AudioSocket AudioSocketConfig
	OpenAI      OpenAIConfig
	Deepgram    DeepgramConfig
	AssemblyAI  AssemblyAIConfig
	ElevenLabs  ElevenLabsConfig
}

// AudioSocketConfig describe el servidor TCP que recibe audio slin de Asterisk
// vía res_audiosocket. Reemplaza el path RTP+External Media para evitar el
// transcoding del bridge.
type AudioSocketConfig struct {
	Enabled bool
	Port    string // string para que coincida con el resto del config (no int)
}

type RTPConfig struct {
	PortStart      int
	PortEnd        int
	AdvertiseHost  string // What Asterisk should use to reach us. e.g. "voice-agent" inside compose, or a public IP.
	Format         string // Formato del External Media de Asterisk: slin16 (default) | ulaw | alaw. DEBE coincidir con el `format` que pasa el backend a Asterisk.
}

type OpenAIConfig struct {
	APIKey string
	Model  string
	Voice  string
}

// DeepgramConfig drives the Voice Agent socket (wss://agent.deepgram.com/v1/agent/converse).
// Deepgram orchestrates ASR + LLM + TTS internally; we only specify which models to use.
type DeepgramConfig struct {
	APIKey         string
	ListenModel    string // ASR — nova-3, flux-general-en, ...
	ThinkProvider  string // open_ai, anthropic, ... (the LLM vendor Deepgram should call)
	ThinkModel     string // gpt-4o-mini, claude-3-5-sonnet-latest, ...
	SpeakModel     string // aura-asteria-en, ...
	Greeting       string
}

// AssemblyAIConfig drives the Voice Agent socket (wss://agents.assemblyai.com/v1/ws).
// AssemblyAI hosts the LLM + TTS internally — we just pick the voice.
type AssemblyAIConfig struct {
	APIKey   string
	Voice    string // ivy, james, tyler, ...
	Greeting string
}

// ElevenLabsConfig conecta con la API de Conversational AI Agents
// (wss://api.elevenlabs.io/v1/convai/conversation?agent_id=...).
// El AGENTE se configura PREVIAMENTE en el dashboard de ElevenLabs
// (voz, system prompt, LLM, tools) — nosotros solo pasamos su id y
// overrideamos lo que haga falta por sesión.
type ElevenLabsConfig struct {
	APIKey  string // key para signed URLs (agentes privados)
	AgentID string // default; el bot puede tener su propio agent_id
}

func Load() (Config, error) {
	openaiKey := env("OPENAI_API_KEY", "")
	cfg := Config{
		Port:           env("VOICE_AGENT_PORT", "8090"),
		BackendURL:     env("BACKEND_URL", "http://backend:8080"),
		BackendAuthKey: env("VOICE_AGENT_SHARED_SECRET", ""),
		AllowedOrigins: splitCSV(env("ALLOWED_ORIGINS", "*")),
		SessionTTL:     envDuration("SESSION_TTL", 30*time.Minute),

		RTP: RTPConfig{
			PortStart:     envInt("RTP_PORT_START", 12000),
			PortEnd:       envInt("RTP_PORT_END", 12099),
			AdvertiseHost: env("RTP_ADVERTISE_HOST", "voice-agent"),
			Format:        env("EXTERNAL_MEDIA_FORMAT", "ulaw"),
		},
		AudioSocket: AudioSocketConfig{
			Enabled: envBool("AUDIOSOCKET_ENABLED", true),
			Port:    env("AUDIOSOCKET_PORT", "9092"),
		},

		OpenAI: OpenAIConfig{
			APIKey: openaiKey,
			// gpt-realtime es el modelo GA (agosto 2025), reemplaza al preview.
			// 20% más barato y mejor instruction following / tool use.
			Model: env("OPENAI_REALTIME_MODEL", "gpt-realtime"),
			// Voz default "alloy" — compatible con todas las cuentas.
			// Las voces nuevas exclusivas de gpt-realtime ("marin", "cedar")
			// están disponibles vía OPENAI_REALTIME_VOICE pero pueden no
			// estar habilitadas en todas las API keys → el bot se queda
			// sin hablar si la voz no se resuelve.
			Voice: env("OPENAI_REALTIME_VOICE", "alloy"),
		},
		Deepgram: DeepgramConfig{
			APIKey:        env("DEEPGRAM_API_KEY", ""),
			ListenModel:   env("DEEPGRAM_LISTEN_MODEL", "nova-3"),
			ThinkProvider: env("DEEPGRAM_THINK_PROVIDER", "open_ai"),
			ThinkModel:    env("DEEPGRAM_THINK_MODEL", "gpt-4o-mini"),
			SpeakModel:    env("DEEPGRAM_SPEAK_MODEL", "aura-asteria-en"),
			Greeting:      env("DEEPGRAM_GREETING", ""),
		},
		AssemblyAI: AssemblyAIConfig{
			APIKey:   env("ASSEMBLYAI_API_KEY", ""),
			Voice:    env("ASSEMBLYAI_VOICE", "ivy"),
			Greeting: env("ASSEMBLYAI_GREETING", ""),
		},
		ElevenLabs: ElevenLabsConfig{
			APIKey:  env("ELEVENLABS_API_KEY", ""),
			AgentID: env("ELEVENLABS_AGENT_ID", ""),
		},
	}
	_ = openaiKey // OpenAI key still consumed by the OpenAI provider via cfg.OpenAI.APIKey.

	if cfg.Port == "" {
		return cfg, errors.New("VOICE_AGENT_PORT required")
	}
	return cfg, nil
}

func env(key, fallback string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	return v
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	n := 0
	for _, c := range v {
		if c < '0' || c > '9' {
			return fallback
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func splitCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

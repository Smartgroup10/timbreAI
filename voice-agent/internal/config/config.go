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

	RTP        RTPConfig
	OpenAI     OpenAIConfig
	Deepgram   DeepgramConfig
	AssemblyAI AssemblyAIConfig
}

type RTPConfig struct {
	PortStart      int
	PortEnd        int
	AdvertiseHost  string // What Asterisk should use to reach us. e.g. "voice-agent" inside compose, or a public IP.
}

type OpenAIConfig struct {
	APIKey string
	Model  string
	Voice  string
}

type DeepgramConfig struct {
	APIKey   string
	ASRModel string
	TTSModel string
	LLMURL   string // Reuses OpenAI Chat Completions; falls back to OPENAI_API_KEY.
	LLMKey   string
	LLMModel string
}

type AssemblyAIConfig struct {
	APIKey   string
	LLMURL   string
	LLMKey   string
	LLMModel string
	TTSURL   string // Defaults to OpenAI speech endpoint.
	TTSKey   string
	TTSModel string
	TTSVoice string
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
		},

		OpenAI: OpenAIConfig{
			APIKey: openaiKey,
			Model:  env("OPENAI_REALTIME_MODEL", "gpt-4o-realtime-preview-2024-12-17"),
			Voice:  env("OPENAI_REALTIME_VOICE", "alloy"),
		},
		Deepgram: DeepgramConfig{
			APIKey:   env("DEEPGRAM_API_KEY", ""),
			ASRModel: env("DEEPGRAM_ASR_MODEL", "nova-3"),
			TTSModel: env("DEEPGRAM_TTS_MODEL", "aura-asteria-en"),
			LLMURL:   env("DEEPGRAM_LLM_URL", "https://api.openai.com/v1/chat/completions"),
			LLMKey:   firstNonEmpty(env("DEEPGRAM_LLM_KEY", ""), openaiKey),
			LLMModel: env("DEEPGRAM_LLM_MODEL", "gpt-4o-mini"),
		},
		AssemblyAI: AssemblyAIConfig{
			APIKey:   env("ASSEMBLYAI_API_KEY", ""),
			LLMURL:   env("ASSEMBLYAI_LLM_URL", "https://api.openai.com/v1/chat/completions"),
			LLMKey:   firstNonEmpty(env("ASSEMBLYAI_LLM_KEY", ""), openaiKey),
			LLMModel: env("ASSEMBLYAI_LLM_MODEL", "gpt-4o-mini"),
			TTSURL:   env("ASSEMBLYAI_TTS_URL", "https://api.openai.com/v1/audio/speech"),
			TTSKey:   firstNonEmpty(env("ASSEMBLYAI_TTS_KEY", ""), openaiKey),
			TTSModel: env("ASSEMBLYAI_TTS_MODEL", "gpt-4o-mini-tts"),
			TTSVoice: env("ASSEMBLYAI_TTS_VOICE", "alloy"),
		},
	}

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

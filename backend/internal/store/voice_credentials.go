package store

import (
	"context"
	"strings"
)

// VoiceCredentials holds per-tenant overrides for voice provider API keys + models. Empty
// strings mean "use the voice-agent's global env default".
//
// All three providers are single-WebSocket Voice Agent APIs — no separate ASR/LLM/TTS keys.
type VoiceCredentials struct {
	TenantID string `json:"tenantId"`

	// OpenAI Realtime: one model and voice.
	OpenAIAPIKey        string `json:"openaiApiKey"`
	OpenAIRealtimeModel string `json:"openaiRealtimeModel"`
	OpenAIRealtimeVoice string `json:"openaiRealtimeVoice"`

	// Deepgram Voice Agent: separate "listen" (ASR), "think" (LLM) and "speak" (TTS) providers,
	// but all delivered over a single WS that Deepgram orchestrates.
	DeepgramAPIKey        string `json:"deepgramApiKey"`
	DeepgramListenModel   string `json:"deepgramListenModel"`   // nova-3, flux-general-en, ...
	DeepgramThinkProvider string `json:"deepgramThinkProvider"` // open_ai, anthropic, ...
	DeepgramThinkModel    string `json:"deepgramThinkModel"`    // gpt-4o-mini, claude-3-5-sonnet-latest, ...
	DeepgramSpeakModel    string `json:"deepgramSpeakModel"`    // aura-asteria-en, ...
	DeepgramGreeting      string `json:"deepgramGreeting"`      // optional opening line

	// AssemblyAI Voice Agent hosts LLM + TTS internally — only API key + voice + greeting.
	AssemblyAIAPIKey   string `json:"assemblyaiApiKey"`
	AssemblyAIVoice    string `json:"assemblyaiVoice"` // ivy, james, tyler, ...
	AssemblyAIGreeting string `json:"assemblyaiGreeting"`
}

// GetVoiceCredentials returns the row, lazily creating defaults if it's missing.
func (s *Store) GetVoiceCredentials(ctx context.Context, tenantID string) (VoiceCredentials, error) {
	var c VoiceCredentials
	err := s.pool.QueryRow(ctx, `
		WITH ensured AS (
		  INSERT INTO tenant_voice_credentials (tenant_id)
		  VALUES ($1) ON CONFLICT DO NOTHING RETURNING tenant_id
		)
		SELECT tenant_id,
		       openai_api_key, openai_realtime_model, openai_realtime_voice,
		       deepgram_api_key, deepgram_listen_model, deepgram_think_provider,
		       deepgram_think_model, deepgram_speak_model, deepgram_greeting,
		       assemblyai_api_key, assemblyai_voice, assemblyai_greeting
		FROM tenant_voice_credentials WHERE tenant_id = $1`, tenantID).
		Scan(&c.TenantID,
			&c.OpenAIAPIKey, &c.OpenAIRealtimeModel, &c.OpenAIRealtimeVoice,
			&c.DeepgramAPIKey, &c.DeepgramListenModel, &c.DeepgramThinkProvider,
			&c.DeepgramThinkModel, &c.DeepgramSpeakModel, &c.DeepgramGreeting,
			&c.AssemblyAIAPIKey, &c.AssemblyAIVoice, &c.AssemblyAIGreeting)
	return c, err
}

type VoiceCredentialsPatch struct {
	OpenAIAPIKey        *string `json:"openaiApiKey,omitempty"`
	OpenAIRealtimeModel *string `json:"openaiRealtimeModel,omitempty"`
	OpenAIRealtimeVoice *string `json:"openaiRealtimeVoice,omitempty"`

	DeepgramAPIKey        *string `json:"deepgramApiKey,omitempty"`
	DeepgramListenModel   *string `json:"deepgramListenModel,omitempty"`
	DeepgramThinkProvider *string `json:"deepgramThinkProvider,omitempty"`
	DeepgramThinkModel    *string `json:"deepgramThinkModel,omitempty"`
	DeepgramSpeakModel    *string `json:"deepgramSpeakModel,omitempty"`
	DeepgramGreeting      *string `json:"deepgramGreeting,omitempty"`

	AssemblyAIAPIKey   *string `json:"assemblyaiApiKey,omitempty"`
	AssemblyAIVoice    *string `json:"assemblyaiVoice,omitempty"`
	AssemblyAIGreeting *string `json:"assemblyaiGreeting,omitempty"`
}

func (s *Store) UpdateVoiceCredentials(ctx context.Context, tenantID string, p VoiceCredentialsPatch) (VoiceCredentials, error) {
	if _, err := s.GetVoiceCredentials(ctx, tenantID); err != nil {
		return VoiceCredentials{}, err
	}
	set := []string{"updated_at = now()"}
	args := []any{tenantID}
	add := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, strings.TrimSpace(*val))
		set = append(set, col+" = $"+itoaCheap(len(args)))
	}
	add("openai_api_key", p.OpenAIAPIKey)
	add("openai_realtime_model", p.OpenAIRealtimeModel)
	add("openai_realtime_voice", p.OpenAIRealtimeVoice)
	add("deepgram_api_key", p.DeepgramAPIKey)
	add("deepgram_listen_model", p.DeepgramListenModel)
	add("deepgram_think_provider", p.DeepgramThinkProvider)
	add("deepgram_think_model", p.DeepgramThinkModel)
	add("deepgram_speak_model", p.DeepgramSpeakModel)
	add("deepgram_greeting", p.DeepgramGreeting)
	add("assemblyai_api_key", p.AssemblyAIAPIKey)
	add("assemblyai_voice", p.AssemblyAIVoice)
	add("assemblyai_greeting", p.AssemblyAIGreeting)
	q := "UPDATE tenant_voice_credentials SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1"
	if _, err := s.pool.Exec(ctx, q, args...); err != nil {
		return VoiceCredentials{}, err
	}
	return s.GetVoiceCredentials(ctx, tenantID)
}

func (c VoiceCredentials) Masked() VoiceCredentials {
	mc := c
	mc.OpenAIAPIKey = maskSecret(c.OpenAIAPIKey)
	mc.DeepgramAPIKey = maskSecret(c.DeepgramAPIKey)
	mc.AssemblyAIAPIKey = maskSecret(c.AssemblyAIAPIKey)
	return mc
}

func maskSecret(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if len(s) <= 4 {
		return "••••"
	}
	return "••••••••" + s[len(s)-4:]
}

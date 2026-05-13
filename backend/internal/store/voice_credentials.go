package store

import (
	"context"
	"strings"
)

// VoiceCredentials holds per-tenant overrides for voice provider API keys + models. Empty
// strings mean "use the voice-agent's global env default".
type VoiceCredentials struct {
	TenantID string `json:"tenantId"`

	OpenAIAPIKey        string `json:"openaiApiKey"`
	OpenAIRealtimeModel string `json:"openaiRealtimeModel"`
	OpenAIRealtimeVoice string `json:"openaiRealtimeVoice"`

	DeepgramAPIKey   string `json:"deepgramApiKey"`
	DeepgramASRModel string `json:"deepgramAsrModel"`
	DeepgramTTSModel string `json:"deepgramTtsModel"`
	DeepgramLLMModel string `json:"deepgramLlmModel"`

	AssemblyAIAPIKey   string `json:"assemblyaiApiKey"`
	AssemblyAILLMModel string `json:"assemblyaiLlmModel"`
	AssemblyAITTSModel string `json:"assemblyaiTtsModel"`
	AssemblyAITTSVoice string `json:"assemblyaiTtsVoice"`
}

// GetVoiceCredentials returns the row, lazily creating defaults if it's missing.
func (s *Store) GetVoiceCredentials(ctx context.Context, tenantID string) (VoiceCredentials, error) {
	var c VoiceCredentials
	err := s.pool.QueryRow(ctx, `
		WITH ensured AS (
		  INSERT INTO tenant_voice_credentials (tenant_id)
		  VALUES ($1) ON CONFLICT DO NOTHING RETURNING tenant_id
		)
		SELECT tenant_id, openai_api_key, openai_realtime_model, openai_realtime_voice,
		       deepgram_api_key, deepgram_asr_model, deepgram_tts_model, deepgram_llm_model,
		       assemblyai_api_key, assemblyai_llm_model, assemblyai_tts_model, assemblyai_tts_voice
		FROM tenant_voice_credentials WHERE tenant_id = $1`, tenantID).
		Scan(&c.TenantID, &c.OpenAIAPIKey, &c.OpenAIRealtimeModel, &c.OpenAIRealtimeVoice,
			&c.DeepgramAPIKey, &c.DeepgramASRModel, &c.DeepgramTTSModel, &c.DeepgramLLMModel,
			&c.AssemblyAIAPIKey, &c.AssemblyAILLMModel, &c.AssemblyAITTSModel, &c.AssemblyAITTSVoice)
	return c, err
}

// VoiceCredentialsPatch lets callers update individual fields. We treat empty string explicitly
// as "clear" so a tenant can remove a key. Use *string to distinguish "not provided" from "set
// to empty".
type VoiceCredentialsPatch struct {
	OpenAIAPIKey        *string `json:"openaiApiKey,omitempty"`
	OpenAIRealtimeModel *string `json:"openaiRealtimeModel,omitempty"`
	OpenAIRealtimeVoice *string `json:"openaiRealtimeVoice,omitempty"`
	DeepgramAPIKey      *string `json:"deepgramApiKey,omitempty"`
	DeepgramASRModel    *string `json:"deepgramAsrModel,omitempty"`
	DeepgramTTSModel    *string `json:"deepgramTtsModel,omitempty"`
	DeepgramLLMModel    *string `json:"deepgramLlmModel,omitempty"`
	AssemblyAIAPIKey    *string `json:"assemblyaiApiKey,omitempty"`
	AssemblyAILLMModel  *string `json:"assemblyaiLlmModel,omitempty"`
	AssemblyAITTSModel  *string `json:"assemblyaiTtsModel,omitempty"`
	AssemblyAITTSVoice  *string `json:"assemblyaiTtsVoice,omitempty"`
}

func (s *Store) UpdateVoiceCredentials(ctx context.Context, tenantID string, p VoiceCredentialsPatch) (VoiceCredentials, error) {
	// Ensure row exists.
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
	add("deepgram_asr_model", p.DeepgramASRModel)
	add("deepgram_tts_model", p.DeepgramTTSModel)
	add("deepgram_llm_model", p.DeepgramLLMModel)
	add("assemblyai_api_key", p.AssemblyAIAPIKey)
	add("assemblyai_llm_model", p.AssemblyAILLMModel)
	add("assemblyai_tts_model", p.AssemblyAITTSModel)
	add("assemblyai_tts_voice", p.AssemblyAITTSVoice)
	q := "UPDATE tenant_voice_credentials SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1"
	if _, err := s.pool.Exec(ctx, q, args...); err != nil {
		return VoiceCredentials{}, err
	}
	return s.GetVoiceCredentials(ctx, tenantID)
}

// Masked returns a copy with API keys redacted (last 4 chars visible). Used in API responses so
// the UI never re-shows full keys after they've been saved.
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

package store

import (
	"context"
	"fmt"
	"strings"

	"timbre/backend/internal/secretcrypto"
)

// VoiceCredentials holds per-tenant overrides for voice provider API keys + models. Empty
// strings mean "use the voice-agent's global env default".
//
// Las API keys (OpenAI/Deepgram/AssemblyAI) se persisten cifradas con AES-256-GCM
// (columnas *_api_key_enc BYTEA). El struct expuesto sigue manejando strings —
// el cifrado/descifrado es transparente dentro de GetVoiceCredentials /
// UpdateVoiceCredentials.
type VoiceCredentials struct {
	TenantID string `json:"tenantId"`

	// OpenAI Realtime: one model and voice.
	OpenAIAPIKey        string `json:"openaiApiKey"`
	OpenAIRealtimeModel string `json:"openaiRealtimeModel"`
	OpenAIRealtimeVoice string `json:"openaiRealtimeVoice"`

	// Deepgram Voice Agent: separate "listen" (ASR), "think" (LLM) and "speak" (TTS) providers,
	// but all delivered over a single WS that Deepgram orchestrates.
	DeepgramAPIKey        string `json:"deepgramApiKey"`
	DeepgramListenModel   string `json:"deepgramListenModel"`
	DeepgramThinkProvider string `json:"deepgramThinkProvider"`
	DeepgramThinkModel    string `json:"deepgramThinkModel"`
	DeepgramSpeakModel    string `json:"deepgramSpeakModel"`
	DeepgramGreeting      string `json:"deepgramGreeting"`

	// AssemblyAI Voice Agent hosts LLM + TTS internally — only API key + voice + greeting.
	AssemblyAIAPIKey   string `json:"assemblyaiApiKey"`
	AssemblyAIVoice    string `json:"assemblyaiVoice"`
	AssemblyAIGreeting string `json:"assemblyaiGreeting"`

	// ElevenLabs Conversational AI — solo la API key. La voz, system
	// prompt, LLM y tools se gestionan en el dashboard de ElevenLabs,
	// y cada bot apunta a un agent_id concreto (Bot.ElevenLabsAgentID).
	ElevenLabsAPIKey string `json:"elevenlabsApiKey"`
}

// GetVoiceCredentials returns the row, lazily creating defaults if it's missing.
// Descifra las API keys con la master key de Store.
func (s *Store) GetVoiceCredentials(ctx context.Context, tenantID string) (VoiceCredentials, error) {
	var c VoiceCredentials
	var openaiEnc, deepgramEnc, assemblyEnc, elevenlabsEnc []byte
	err := s.pool.QueryRow(ctx, `
		WITH ensured AS (
		  INSERT INTO tenant_voice_credentials (tenant_id)
		  VALUES ($1) ON CONFLICT DO NOTHING RETURNING tenant_id
		)
		SELECT tenant_id,
		       openai_api_key_enc, openai_realtime_model, openai_realtime_voice,
		       deepgram_api_key_enc, deepgram_listen_model, deepgram_think_provider,
		       deepgram_think_model, deepgram_speak_model, deepgram_greeting,
		       assemblyai_api_key_enc, assemblyai_voice, assemblyai_greeting,
		       elevenlabs_api_key_enc
		FROM tenant_voice_credentials WHERE tenant_id = $1`, tenantID).
		Scan(&c.TenantID,
			&openaiEnc, &c.OpenAIRealtimeModel, &c.OpenAIRealtimeVoice,
			&deepgramEnc, &c.DeepgramListenModel, &c.DeepgramThinkProvider,
			&c.DeepgramThinkModel, &c.DeepgramSpeakModel, &c.DeepgramGreeting,
			&assemblyEnc, &c.AssemblyAIVoice, &c.AssemblyAIGreeting,
			&elevenlabsEnc)
	if err != nil {
		return c, err
	}
	if c.OpenAIAPIKey, err = s.decryptOrEmpty(openaiEnc); err != nil {
		return c, fmt.Errorf("decrypt openai key: %w", err)
	}
	if c.DeepgramAPIKey, err = s.decryptOrEmpty(deepgramEnc); err != nil {
		return c, fmt.Errorf("decrypt deepgram key: %w", err)
	}
	if c.AssemblyAIAPIKey, err = s.decryptOrEmpty(assemblyEnc); err != nil {
		return c, fmt.Errorf("decrypt assemblyai key: %w", err)
	}
	if c.ElevenLabsAPIKey, err = s.decryptOrEmpty(elevenlabsEnc); err != nil {
		return c, fmt.Errorf("decrypt elevenlabs key: %w", err)
	}
	return c, nil
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

	ElevenLabsAPIKey *string `json:"elevenlabsApiKey,omitempty"`
}

func (s *Store) UpdateVoiceCredentials(ctx context.Context, tenantID string, p VoiceCredentialsPatch) (VoiceCredentials, error) {
	if _, err := s.GetVoiceCredentials(ctx, tenantID); err != nil {
		return VoiceCredentials{}, err
	}
	set := []string{"updated_at = now()"}
	args := []any{tenantID}
	addText := func(col string, val *string) {
		if val == nil {
			return
		}
		args = append(args, strings.TrimSpace(*val))
		set = append(set, col+" = $"+itoaCheap(len(args)))
	}
	addSecret := func(col string, val *string) error {
		if val == nil {
			return nil
		}
		plain := strings.TrimSpace(*val)
		var enc []byte
		if plain != "" {
			ct, err := secretcrypto.Encrypt(s.secretsKey, []byte(plain))
			if err != nil {
				return err
			}
			enc = ct
		}
		args = append(args, enc)
		set = append(set, col+" = $"+itoaCheap(len(args)))
		return nil
	}
	if err := addSecret("openai_api_key_enc", p.OpenAIAPIKey); err != nil {
		return VoiceCredentials{}, fmt.Errorf("encrypt openai key: %w", err)
	}
	addText("openai_realtime_model", p.OpenAIRealtimeModel)
	addText("openai_realtime_voice", p.OpenAIRealtimeVoice)
	if err := addSecret("deepgram_api_key_enc", p.DeepgramAPIKey); err != nil {
		return VoiceCredentials{}, fmt.Errorf("encrypt deepgram key: %w", err)
	}
	addText("deepgram_listen_model", p.DeepgramListenModel)
	addText("deepgram_think_provider", p.DeepgramThinkProvider)
	addText("deepgram_think_model", p.DeepgramThinkModel)
	addText("deepgram_speak_model", p.DeepgramSpeakModel)
	addText("deepgram_greeting", p.DeepgramGreeting)
	if err := addSecret("assemblyai_api_key_enc", p.AssemblyAIAPIKey); err != nil {
		return VoiceCredentials{}, fmt.Errorf("encrypt assemblyai key: %w", err)
	}
	addText("assemblyai_voice", p.AssemblyAIVoice)
	addText("assemblyai_greeting", p.AssemblyAIGreeting)
	if err := addSecret("elevenlabs_api_key_enc", p.ElevenLabsAPIKey); err != nil {
		return VoiceCredentials{}, fmt.Errorf("encrypt elevenlabs key: %w", err)
	}
	q := "UPDATE tenant_voice_credentials SET " + strings.Join(set, ", ") + " WHERE tenant_id = $1"
	if _, err := s.pool.Exec(ctx, q, args...); err != nil {
		return VoiceCredentials{}, err
	}
	return s.GetVoiceCredentials(ctx, tenantID)
}

// decryptOrEmpty descifra el blob; si está vacío devuelve "" sin error. Datos
// corruptos (key rotada, blob malformado) sí devuelven error — preferimos
// fallar a devolver basura.
func (s *Store) decryptOrEmpty(enc []byte) (string, error) {
	if len(enc) == 0 {
		return "", nil
	}
	plain, err := secretcrypto.Decrypt(s.secretsKey, enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

func (c VoiceCredentials) Masked() VoiceCredentials {
	mc := c
	mc.OpenAIAPIKey = maskSecret(c.OpenAIAPIKey)
	mc.DeepgramAPIKey = maskSecret(c.DeepgramAPIKey)
	mc.AssemblyAIAPIKey = maskSecret(c.AssemblyAIAPIKey)
	mc.ElevenLabsAPIKey = maskSecret(c.ElevenLabsAPIKey)
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

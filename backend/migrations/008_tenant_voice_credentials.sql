-- Per-tenant overrides for voice provider API keys and models. When set, these win over the
-- global env vars in the voice-agent service. Empty strings = fall back to env defaults.
-- Stored as plaintext in this version; encrypting at rest with a master key is a TODO.
CREATE TABLE IF NOT EXISTS tenant_voice_credentials (
  tenant_id              TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,

  openai_api_key         TEXT NOT NULL DEFAULT '',
  openai_realtime_model  TEXT NOT NULL DEFAULT '',
  openai_realtime_voice  TEXT NOT NULL DEFAULT '',

  deepgram_api_key       TEXT NOT NULL DEFAULT '',
  deepgram_asr_model     TEXT NOT NULL DEFAULT '',
  deepgram_tts_model     TEXT NOT NULL DEFAULT '',
  deepgram_llm_model     TEXT NOT NULL DEFAULT '',

  assemblyai_api_key     TEXT NOT NULL DEFAULT '',
  assemblyai_llm_model   TEXT NOT NULL DEFAULT '',
  assemblyai_tts_model   TEXT NOT NULL DEFAULT '',
  assemblyai_tts_voice   TEXT NOT NULL DEFAULT '',

  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed empty rows for existing tenants so the GET endpoint always returns something.
INSERT INTO tenant_voice_credentials (tenant_id)
SELECT id FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;

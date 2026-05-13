-- Switch credentials schema to match the Voice Agent end-to-end APIs.
-- Deepgram Voice Agent (wss://agent.deepgram.com/v1/agent/converse) packs listen+think+speak in
-- one socket — what used to be ASR/TTS/LLM models are now "listen", "speak", "think" with an
-- explicit think.provider type.
-- AssemblyAI Voice Agent (wss://agents.assemblyai.com/v1/ws) handles LLM+TTS internally, so we
-- only need the API key + voice + optional greeting.

ALTER TABLE tenant_voice_credentials RENAME COLUMN deepgram_asr_model TO deepgram_listen_model;
ALTER TABLE tenant_voice_credentials RENAME COLUMN deepgram_tts_model TO deepgram_speak_model;
ALTER TABLE tenant_voice_credentials RENAME COLUMN deepgram_llm_model TO deepgram_think_model;
ALTER TABLE tenant_voice_credentials ADD COLUMN IF NOT EXISTS deepgram_think_provider TEXT NOT NULL DEFAULT 'open_ai';
ALTER TABLE tenant_voice_credentials ADD COLUMN IF NOT EXISTS deepgram_greeting TEXT NOT NULL DEFAULT '';

ALTER TABLE tenant_voice_credentials RENAME COLUMN assemblyai_tts_voice TO assemblyai_voice;
ALTER TABLE tenant_voice_credentials DROP COLUMN IF EXISTS assemblyai_llm_model;
ALTER TABLE tenant_voice_credentials DROP COLUMN IF EXISTS assemblyai_tts_model;
ALTER TABLE tenant_voice_credentials ADD COLUMN IF NOT EXISTS assemblyai_greeting TEXT NOT NULL DEFAULT '';

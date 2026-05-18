-- 030_elevenlabs.sql
-- Soporte de ElevenLabs Conversational AI Agents como cuarto provider de voz.
--
-- A diferencia de OpenAI/Deepgram/AssemblyAI, el agente de ElevenLabs se
-- pre-configura en SU dashboard (voz, system prompt, LLM, tools). Nosotros
-- conectamos con su agent_id y opcionalmente overrideamos algunos campos.
--
-- Por eso necesitamos dos cosas:
--   - bots.elevenlabs_agent_id  → cada bot apunta a su agente concreto
--   - tenant_voice_credentials.elevenlabs_api_key → key para agentes privados
--
-- Pricing: Voice Agent API de ElevenLabs cobra $0.08-$0.12/min según el
-- LLM del agente (GPT-4o premium = 12c, voice-only = 8c). Default 10 c/min,
-- componentes de provider_rates a 0 → fallback al flat (mismo patrón que
-- Deepgram y AssemblyAI).
--
-- Fuente: https://elevenlabs.io/pricing/agents

ALTER TABLE bots
  ADD COLUMN IF NOT EXISTS elevenlabs_agent_id text NOT NULL DEFAULT '';

ALTER TABLE tenant_voice_credentials
  ADD COLUMN IF NOT EXISTS elevenlabs_api_key_enc BYTEA NOT NULL DEFAULT '';

INSERT INTO provider_rates (provider, component, unit, micro_cents_per_unit) VALUES
  ('elevenlabs', 'stt',        'sec',      0),
  ('elevenlabs', 'llm_input',  '1k_token', 0),
  ('elevenlabs', 'llm_output', '1k_token', 0),
  ('elevenlabs', 'tts',        'char',     0)
ON CONFLICT (provider, component) DO NOTHING;

-- 029_assemblyai_voice_agent_flat.sql
-- Corrección de tarifas de AssemblyAI Voice Agent API.
--
-- AssemblyAI Voice Agent (igual que Deepgram) es flat por minuto, no por
-- componente STT/LLM/TTS. Tarifa actual (precio público):
--
--   Voice Agent API:  $4.50/hr  =  $0.075/min  Pay As You Go
--   Custom:           tarifas por contrato (contact sales)
--
--   Fuente: https://www.assemblyai.com/pricing
--           https://www.assemblyai.com/docs/voice-agents/voice-agent-api
--
-- Mismo tratamiento que con Deepgram: ponemos los componentes a 0 para
-- forzar el fallback al flat cents/min del paquete pricing
-- (PRICING_ASSEMBLYAI_CENTS_PER_MIN, default 8 c/min).
--
-- Las tarifas seed de 026 (stt 15 μ¢/sec, tts 2 μ¢/char) eran números
-- sin base oficial.

UPDATE provider_rates SET
  micro_cents_per_unit = 0,
  updated_at = now()
WHERE provider = 'assemblyai'
  AND component IN ('stt', 'llm_input', 'llm_output', 'tts');

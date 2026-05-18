-- 028_deepgram_voice_agent_flat.sql
-- Corrección de tarifas de Deepgram Voice Agent API.
--
-- El Voice Agent API de Deepgram NO factura por componente (STT/LLM/TTS)
-- como asumían las tarifas seed de 026. Es un único precio flat por
-- minuto que depende del tier:
--
--   Pay As You Go (sin compromiso):
--     - Standard:                $0.075/min
--     - Standard BYO TTS:        $0.065/min
--     - Custom (BYO LLM):        $0.056/min
--     - Custom (BYO LLM + TTS):  $0.050/min
--     - Advanced:                $0.163/min
--     - Advanced BYO TTS:        $0.122/min
--
--   Growth tier (con compromiso de volumen):
--     - Standard:                $0.068/min
--     - Custom (BYO LLM + TTS):  $0.041/min
--     - Advanced:                $0.146/min
--
--   Fuente: https://deepgram.com/pricing
--
-- Nuestra integración usa "Custom — BYO LLM" porque pasamos la OpenAI key
-- del tenant a Deepgram via endpoint.headers.authorization. NOTA: además
-- del precio Deepgram, el operador paga aparte el LLM directamente a
-- OpenAI (Anthropic/etc.) — eso queda fuera de la tarifa Deepgram.
--
-- Los componentes (stt/llm_input/llm_output/tts) los ponemos a 0 para
-- forzar el fallback al flat cents/min del paquete pricing. Cuando el
-- voice-agent reporta usage al cerrar la sesión, OtherMicroCents recibe
-- el flat × duration (ver handlers_billing.go).

UPDATE provider_rates SET
  micro_cents_per_unit = 0,
  updated_at = now()
WHERE provider = 'deepgram'
  AND component IN ('stt', 'llm_input', 'llm_output', 'tts');

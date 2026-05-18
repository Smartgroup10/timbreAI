-- 027_openai_realtime_ga.sql
-- Actualización de tarifas a gpt-realtime GA (agosto 2025).
--
-- OpenAI hace gpt-realtime disponible en GA con un 20% de descuento
-- respecto a gpt-4o-realtime-preview. Las tarifas seed de 026 estaban
-- en valores de tokens de texto, no de audio — el modelo realtime
-- factura TODO el audio (in + out) como tokens de audio, no como STT/TTS
-- separados.
--
-- Tarifas oficiales gpt-realtime (anuncio OpenAI):
--   - Audio input:   $32 / 1M tokens     → 32,000 μ¢ por 1k tokens
--   - Audio output:  $64 / 1M tokens     → 64,000 μ¢ por 1k tokens
--   - Cached input:  $0.40 / 1M tokens   →    400 μ¢ por 1k tokens
--
-- Fuente: https://openai.com/index/introducing-gpt-realtime/
--         https://developers.openai.com/api/docs/guides/realtime
--
-- Mapping conceptual:
--   llm_input         → audio input tokens (lo que dice el caller)
--   llm_output        → audio output tokens (lo que dice el bot)
--   tts               → 0  (no hay TTS separada en realtime, va en audio out)
--   llm_input_cached  → audio input cacheado (NUEVO — para futuro soporte
--                       de prompt caching de OpenAI)

UPDATE provider_rates SET
  micro_cents_per_unit = 32000,
  updated_at = now()
WHERE provider = 'openai_realtime' AND component = 'llm_input';

UPDATE provider_rates SET
  micro_cents_per_unit = 64000,
  updated_at = now()
WHERE provider = 'openai_realtime' AND component = 'llm_output';

-- En realtime no hay TTS separada: el audio de salida ya está facturado
-- como llm_output. Ponemos 0 para que el cálculo no duplique el coste.
UPDATE provider_rates SET
  micro_cents_per_unit = 0,
  updated_at = now()
WHERE provider = 'openai_realtime' AND component = 'tts';

-- Cached input — nuevo componente. Si en el futuro instrumentamos el
-- voice-agent para reportar cuántos tokens vinieron del cache de OpenAI,
-- el backend ya tendrá la tarifa lista.
INSERT INTO provider_rates (provider, component, unit, micro_cents_per_unit)
VALUES ('openai_realtime', 'llm_input_cached', '1k_token', 400)
ON CONFLICT (provider, component) DO UPDATE SET
  micro_cents_per_unit = EXCLUDED.micro_cents_per_unit,
  updated_at = now();

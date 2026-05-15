-- 026_call_usage.sql
-- Coste real por llamada con breakdown por componente (STT/LLM/TTS/trunk)
-- y dashboard de billing.
--
-- Hasta ahora teníamos pricing.Cost(provider, duration_sec) — una estimación
-- flat por minuto que está bien para "mostrar un número en la fila de
-- llamadas" pero no para:
--   - Reportar al cliente cuánto gasta en cada componente (Deepgram listen
--     es barato pero el LLM hosted es caro; mejor tener visibilidad).
--   - Calcular margen real (qué te cobra el provider vs qué cobras tú).
--   - Detectar campañas runaway que se comen el presupuesto.
--
-- call_usage es 1:1 con calls (no inflamos la fila de calls). El voice-agent
-- reporta los contadores al cerrar la sesión via POST /api/internal/voice/usage
-- y el backend calcula los céntimos usando provider_rates (tabla global) o
-- los defaults del paquete pricing si la tabla está vacía.

CREATE TABLE IF NOT EXISTS call_usage (
  call_id              text PRIMARY KEY REFERENCES calls(id) ON DELETE CASCADE,
  tenant_id            text NOT NULL,
  -- Snapshot del provider y la duración para que el dashboard pueda
  -- agruparse rápidamente sin join a calls (path caliente).
  provider             text NOT NULL DEFAULT '',
  duration_sec         int  NOT NULL DEFAULT 0,

  -- Breakdown por componente. Cualquier campo en 0 significa "no
  -- consumido" o "el provider no nos da esa métrica" (echo, p.ej.).
  stt_seconds          int  NOT NULL DEFAULT 0,
  llm_input_tokens     int  NOT NULL DEFAULT 0,
  llm_output_tokens    int  NOT NULL DEFAULT 0,
  tts_chars            int  NOT NULL DEFAULT 0,
  tts_seconds          int  NOT NULL DEFAULT 0,

  -- Coste calculado en micro-céntimos (1e-6 USD). Granularidad fina
  -- para que sumar 1000 llamadas no pierda decimales por redondeo.
  -- Para presentar al usuario, dividimos por 10000 → centavos enteros.
  stt_micro_cents      bigint NOT NULL DEFAULT 0,
  llm_micro_cents      bigint NOT NULL DEFAULT 0,
  tts_micro_cents      bigint NOT NULL DEFAULT 0,
  trunk_micro_cents    bigint NOT NULL DEFAULT 0,
  other_micro_cents    bigint NOT NULL DEFAULT 0,
  total_micro_cents    bigint NOT NULL DEFAULT 0,

  created_at           timestamptz NOT NULL DEFAULT now(),
  updated_at           timestamptz NOT NULL DEFAULT now()
);

-- Dashboard "gasto del mes" — agrupar por día.
CREATE INDEX IF NOT EXISTS idx_call_usage_tenant_day
  ON call_usage(tenant_id, created_at DESC);

-- Top campañas/bots por coste — requiere join a calls; con este índice
-- por call_id basta para EXISTS / inner join eficiente.

-- Tabla global de tarifas por provider/componente. Si está vacía, el
-- backend cae a los defaults del paquete pricing (cents/min flat). Si
-- tiene filas, las usa preferentemente para granularidad por componente.
-- micro_cents_per_unit: 1e-6 USD por unit. Ej. OpenAI Realtime input
-- $0.06 / 1M tokens = 0.00006 $/1k tokens = 60 micro-cents/1k tokens.
CREATE TABLE IF NOT EXISTS provider_rates (
  provider             text NOT NULL,
  component            text NOT NULL,
  -- 'sec', 'min', 'token', '1k_token', 'char', '1k_char'
  unit                 text NOT NULL,
  micro_cents_per_unit bigint NOT NULL,
  updated_at           timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY (provider, component)
);

-- Seed inicial con las tarifas públicas conocidas. NO se actualizan
-- automáticamente — el operador edita en BD si cambian o las override
-- por env.
INSERT INTO provider_rates (provider, component, unit, micro_cents_per_unit) VALUES
  ('openai_realtime', 'llm_input',  '1k_token', 60),    -- $0.006/1k input tokens
  ('openai_realtime', 'llm_output', '1k_token', 240),   -- $0.024/1k output tokens
  ('openai_realtime', 'tts',        'sec',      100),   -- absorbida en realtime, aprox.
  ('deepgram',        'stt',        'sec',      4),     -- $0.0024/min ≈ 0.04 μcents/sec ≈ 4 picocents/sec → guardamos 4 μc/sec como simplificación visible
  ('deepgram',        'llm_input',  '1k_token', 25),    -- nova-3 think
  ('deepgram',        'llm_output', '1k_token', 100),
  ('deepgram',        'tts',        'char',     1),     -- aura-2 $30/1M chars
  ('assemblyai',      'stt',        'sec',      15),
  ('assemblyai',      'tts',        'char',     2),
  ('echo',            'stt',        'sec',      0)
ON CONFLICT (provider, component) DO NOTHING;

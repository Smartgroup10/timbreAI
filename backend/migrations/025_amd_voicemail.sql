-- 025_amd_voicemail.sql
-- AMD (Answering Machine Detection) + voicemail drop.
--
-- Caso de uso outbound: el bot llama y a veces salta el buzón. Hoy el bot
-- intenta conversar con la grabación del buzón, gasta tokens y cuelga la
-- llamada con datos sucios en el dashboard ("0% conversión hoy" cuando
-- en realidad fueron buzones). Queremos:
--   1. Detectar si el "Hola" inicial es de una persona o de una máquina.
--   2. Según la config del bot, o colgar inmediatamente (no gastar) o
--      soltar un mensaje pre-escrito ("Hola, soy de X, te llamaré
--      mañana", TTS del provider) y colgar.
--   3. Marcar la call con el resultado para filtrar el reporting.
--
-- La detección la hace el voice-agent con una heurística sobre el audio
-- entrante (energy/duración del primer burst de speech). No usa Asterisk
-- AMD() porque tendríamos que detectar antes del Stasis bridge y nuestro
-- flujo es AudioSocket directo. Vive en el voice-agent y se decide
-- después de ~5s.

ALTER TABLE bots
  ADD COLUMN IF NOT EXISTS amd_enabled boolean NOT NULL DEFAULT false,
  -- Acción al detectar buzón:
  --   'hangup'       — cuelga sin gastar TTS
  --   'drop_message' — TTS del campo voicemail_message y cuelga
  --   'continue'     — sigue la conversación normal (legacy/disabled)
  ADD COLUMN IF NOT EXISTS amd_action text NOT NULL DEFAULT 'hangup'
    CHECK (amd_action IN ('hangup', 'drop_message', 'continue')),
  -- Mensaje que recita el bot al detectar buzón (usado si amd_action='drop_message').
  -- Vacío = no recita nada aunque action='drop_message' (degrada a hangup).
  ADD COLUMN IF NOT EXISTS voicemail_message text NOT NULL DEFAULT '';

ALTER TABLE calls
  -- Resultado de la detección. Útil para filtrar el dashboard
  -- ("solo llamadas con humano") y para entrenar mejor el detector.
  --   'human'   — detectado humano (proceso normal)
  --   'machine' — detectado buzón
  --   'unknown' — no se pudo decidir (típicamente llamadas muy cortas)
  --   ''        — AMD deshabilitado o no corrió
  ADD COLUMN IF NOT EXISTS amd_result text NOT NULL DEFAULT '',
  -- True si el bot llegó a soltar el voicemail_message al buzón.
  ADD COLUMN IF NOT EXISTS voicemail_dropped boolean NOT NULL DEFAULT false;

-- Índice para "llamadas que cayeron a buzón hoy" — query del dashboard.
CREATE INDEX IF NOT EXISTS idx_calls_amd_result
  ON calls(tenant_id, amd_result, started_at DESC)
  WHERE amd_result <> '';

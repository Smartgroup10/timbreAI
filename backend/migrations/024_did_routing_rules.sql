-- 024_did_routing_rules.sql
-- Reglas de routing por DID para llamadas entrantes. Patrón inspirado en
-- SmartSIP: cada DID tiene 0..N reglas con prioridad y condiciones (días,
-- horario, prefijo del caller, idioma). Se evalúan ordenadas; la primera
-- que matchea decide qué bot atiende.
--
-- Si no matchea ninguna regla, fallback al bot asignado al DID via
-- bots.did_id (el comportamiento que ya tenía la inbound v1).
--
-- Casos típicos:
--   - "L-V 9:00-18:00 → bot comercial" (priority 100)
--   - "Sábados 10-14 → bot de guardia" (priority 110)
--   - "Llamadas desde +1 → bot inglés" (priority 50, sin restricción horaria)
--   - Fuera de cualquier regla activa: cuelga (o cae al bot default del
--     DID si está configurado).

CREATE TABLE IF NOT EXISTS did_routing_rules (
  id              text PRIMARY KEY,
  tenant_id       text NOT NULL,
  did_id          text NOT NULL REFERENCES dids(id) ON DELETE CASCADE,
  name            text NOT NULL,
  -- Menor número = mayor prioridad. Evaluación: ORDER BY priority ASC,
  -- created_at ASC para tiebreak determinista.
  priority        int  NOT NULL DEFAULT 100,
  enabled         boolean NOT NULL DEFAULT true,

  -- ─── Condiciones de match ───────────────────────────────────────────
  -- Cualquiera de estos NULL = no aplica esa condición.
  --
  -- IANA timezone. La evaluación de start/end/days es en LA HORA LOCAL
  -- del tenant tal y como la ve el caller (p.ej. tienda local en Madrid).
  timezone        text NOT NULL DEFAULT 'Europe/Madrid',
  -- 0=Domingo … 6=Sábado (estilo time.Weekday() de Go). Array vacío = todos los días.
  days_of_week    smallint[] NOT NULL DEFAULT '{}',
  -- Minuto del día en hora local [0..1439]. start_minute=540 (9:00),
  -- end_minute=1080 (18:00). Si start > end es ventana overnight (p.ej.
  -- 22:00 a 06:00). NULL en ambos = sin restricción horaria.
  start_minute    int,
  end_minute      int,
  -- Prefijos del caller que matchean (p.ej. ['+34','+1']). Vacío = cualquiera.
  caller_prefixes text[] NOT NULL DEFAULT '{}',
  -- Código ISO 639-1 ("es", "en"). Empty = cualquiera. Por ahora no
  -- detectamos idioma del caller real, pero dejamos el campo para
  -- routing manual ("si en el dialplan llega ?lang=en", futuro).
  language        text NOT NULL DEFAULT '',

  -- ─── Resultado ──────────────────────────────────────────────────────
  -- Bot que atiende si la regla matchea. ON DELETE RESTRICT — no
  -- permitimos borrar un bot que tiene reglas apuntándole; el operador
  -- las quita primero. Evita reglas "huérfanas" que rutean a la nada.
  target_bot_id   text NOT NULL REFERENCES bots(id) ON DELETE RESTRICT,
  -- Bot fallback opcional si por alguna razón el target no se puede
  -- usar (p.ej. mañana ampliamos para "si target sin credenciales").
  -- Hoy no se usa pero queda registrado en el schema.
  fallback_bot_id text REFERENCES bots(id) ON DELETE SET NULL,

  created_at      timestamptz NOT NULL DEFAULT now(),
  updated_at      timestamptz NOT NULL DEFAULT now(),

  -- Coherencia: start/end o ambos NULL o ambos seteados en rango válido.
  CONSTRAINT did_rules_minutes_check CHECK (
    (start_minute IS NULL AND end_minute IS NULL)
    OR (start_minute BETWEEN 0 AND 1439 AND end_minute BETWEEN 0 AND 1439)
  )
);

-- Lookup por DID (el único path caliente: al recibir cada inbound).
CREATE INDEX IF NOT EXISTS idx_did_rules_did_priority
  ON did_routing_rules(did_id, priority) WHERE enabled = true;

-- Por tenant — admin/listing.
CREATE INDEX IF NOT EXISTS idx_did_rules_tenant
  ON did_routing_rules(tenant_id);

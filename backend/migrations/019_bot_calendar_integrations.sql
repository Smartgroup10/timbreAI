-- 019_bot_calendar_integrations.sql
-- Conexión OAuth a Google Calendar por bot. Cada bot puede pertenecer a
-- un comercial diferente, así que la integración va a nivel bot, no
-- tenant — una inmobiliaria con 5 comerciales tiene 5 bots con 5
-- calendarios distintos.
--
-- refresh_token se almacena cifrado con AES-256-GCM (mismo MASTER_KEY
-- que voice_credentials). access_token también cifrado por uniformidad,
-- aunque su TTL corto lo hace menos crítico.

CREATE TABLE IF NOT EXISTS bot_calendar_integrations (
  id                       text PRIMARY KEY,
  tenant_id                text NOT NULL,
  bot_id                   text NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
  -- 'google' por ahora; 'microsoft' / 'apple' en futuras iteraciones.
  provider                 text NOT NULL DEFAULT 'google',
  -- email de la cuenta conectada, mostrado en la UI ("Conectado como
  -- maria@inmobiliaria.es").
  account_email            text NOT NULL,
  -- "primary" por defecto. Si más adelante damos a elegir un calendar
  -- secundario, se cambia aquí.
  calendar_id              text NOT NULL DEFAULT 'primary',
  -- bytea (no text) porque AES-256-GCM produce binario; mismo patrón
  -- que voice_credentials.*_api_key_enc.
  refresh_token_encrypted  bytea NOT NULL,
  access_token_encrypted   bytea NOT NULL DEFAULT ''::bytea,
  access_token_expires_at  timestamptz,
  scopes                   text NOT NULL DEFAULT '',
  connected_at             timestamptz NOT NULL DEFAULT now(),
  last_used_at             timestamptz,
  updated_at               timestamptz NOT NULL DEFAULT now(),
  -- Un bot solo puede tener una integración con un provider a la vez.
  -- Reconectar reemplaza la existente vía ON CONFLICT.
  CONSTRAINT bot_calendar_provider_unique UNIQUE (bot_id, provider)
);

CREATE INDEX IF NOT EXISTS idx_bot_calendar_tenant
  ON bot_calendar_integrations(tenant_id);

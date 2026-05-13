CREATE TABLE IF NOT EXISTS tenant_settings (
  tenant_id           TEXT PRIMARY KEY REFERENCES tenants(id) ON DELETE CASCADE,
  timezone            TEXT NOT NULL DEFAULT 'Europe/Madrid',
  caller_id_default   TEXT NOT NULL DEFAULT '',
  allowed_hours_start TIME NOT NULL DEFAULT '10:00',
  allowed_hours_end   TIME NOT NULL DEFAULT '18:00',
  allowed_days        TEXT NOT NULL DEFAULT 'mon,tue,wed,thu,fri',
  daily_call_cap      INTEGER NOT NULL DEFAULT 250,
  recording_enabled   BOOLEAN NOT NULL DEFAULT false,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed defaults for existing tenants so the frontend always has something to render.
INSERT INTO tenant_settings (tenant_id)
SELECT id FROM tenants
ON CONFLICT (tenant_id) DO NOTHING;

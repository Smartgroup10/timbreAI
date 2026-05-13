-- SIP trunks managed by platform admins. The actual SIP credentials live in
-- asterisk/etc/asterisk/pjsip.conf (under the [asterisk_endpoint] section).
-- This table is the metadata layer the portal uses to route calls.
CREATE TABLE IF NOT EXISTS sip_trunks (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  provider TEXT NOT NULL DEFAULT '',
  asterisk_endpoint TEXT NOT NULL UNIQUE,
  host TEXT NOT NULL DEFAULT '',
  port INTEGER NOT NULL DEFAULT 5060,
  status TEXT NOT NULL DEFAULT 'active',
  notes TEXT NOT NULL DEFAULT '',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Direct Inward Dialing numbers. Each DID belongs to a trunk and may be assigned
-- to a tenant by the platform admin. Tenants then assign their DIDs to bots.
CREATE TABLE IF NOT EXISTS dids (
  id TEXT PRIMARY KEY,
  trunk_id TEXT NOT NULL REFERENCES sip_trunks(id) ON DELETE RESTRICT,
  tenant_id TEXT REFERENCES tenants(id) ON DELETE SET NULL,
  e164 TEXT NOT NULL UNIQUE,
  label TEXT NOT NULL DEFAULT '',
  status TEXT NOT NULL DEFAULT 'active',
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_dids_tenant ON dids(tenant_id);
CREATE INDEX IF NOT EXISTS idx_dids_trunk ON dids(trunk_id);

-- A bot has at most one outbound DID (its public phone number).
ALTER TABLE bots ADD COLUMN IF NOT EXISTS did_id TEXT REFERENCES dids(id) ON DELETE SET NULL;

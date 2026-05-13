-- Campaign membership: which specific leads belong to which campaign, plus per-lead attempt
-- tracking so the worker can respect max_attempts and cooldowns.
CREATE TABLE IF NOT EXISTS campaign_leads (
  id              TEXT PRIMARY KEY,
  tenant_id       TEXT NOT NULL REFERENCES tenants(id) ON DELETE CASCADE,
  campaign_id     TEXT NOT NULL REFERENCES campaigns(id) ON DELETE CASCADE,
  lead_id         TEXT NOT NULL REFERENCES leads(id) ON DELETE CASCADE,
  status          TEXT NOT NULL DEFAULT 'pending', -- pending, calling, done, failed, blocked
  attempts        INTEGER NOT NULL DEFAULT 0,
  last_attempt_at TIMESTAMPTZ,
  outcome         TEXT NOT NULL DEFAULT '',
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (campaign_id, lead_id)
);

CREATE INDEX IF NOT EXISTS idx_campaign_leads_tenant ON campaign_leads(tenant_id);
CREATE INDEX IF NOT EXISTS idx_campaign_leads_status ON campaign_leads(campaign_id, status);

-- Add a cooldown so we don't retry the same lead too aggressively within one campaign tick.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS retry_cooldown_minutes INTEGER NOT NULL DEFAULT 60;

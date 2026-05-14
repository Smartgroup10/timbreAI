-- 014_call_cost.sql
-- Track which voice provider drove each call so we can compute cost.
--
-- Until now we couldn't tell after the fact what cost a call had: the bot
-- could have its voiceProvider changed afterwards, and the same campaign
-- might have used different bots over time. We snapshot the provider on
-- the call row at creation time, exactly like we already do with
-- lead_name/campaign_name.
--
-- Pricing is config (env vars, see internal/pricing) — not stored in DB.
-- That way you can update rates without a migration and the historical
-- duration × current_rate gives a fair estimate.

ALTER TABLE calls
  ADD COLUMN IF NOT EXISTS provider text NOT NULL DEFAULT '';

-- Index by provider for analytics aggregations.
CREATE INDEX IF NOT EXISTS idx_calls_tenant_provider ON calls(tenant_id, provider);

-- Seed demo data. Idempotent: ON CONFLICT DO NOTHING. Users are bootstrapped in code on first boot.
INSERT INTO tenants (id, name, status, plan) VALUES
  ('atrium', 'Atrium Leasing', 'active', 'platform'),
  ('demo-homes', 'Demo Homes', 'active', 'growth')
ON CONFLICT (id) DO NOTHING;

INSERT INTO leads (id, tenant_id, name, phone, email, type, status, source, consent) VALUES
  ('lead_001', 'atrium', 'Maria Lopez', '+1 555 0101', 'maria@example.com', 'renter', 'qualified', 'webform', 'lead_form'),
  ('lead_002', 'atrium', 'Carlos Rivera', '+1 555 0102', 'carlos@example.com', 'owner', 'new', 'crm', 'existing_lead'),
  ('lead_003', 'atrium', 'Ana Torres', '+1 555 0103', 'ana@example.com', 'renter', 'callback', 'portal', 'lead_form')
ON CONFLICT (id) DO NOTHING;

INSERT INTO properties (id, tenant_id, name, address, price, availability, requirements, faqs) VALUES
  ('prop_001', 'atrium', 'Sunset Villas 2B', 'Miami, FL', '$2,450/mo', 'Available now',
   '["Income 3x rent","Background check","Application fee"]'::jsonb,
   '["Pets allowed with deposit","Parking included"]'::jsonb),
  ('prop_002', 'atrium', 'Downtown Studio', 'Orlando, FL', '$1,650/mo', 'June 1',
   '["Income verification","No evictions"]'::jsonb,
   '["Utilities separate","12 month lease"]'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO bots (id, tenant_id, name, type, language, voice, status, objective, guardrails) VALUES
  ('bot_001', 'atrium', 'Leasing Assistant', 'renter_inbound', 'es-US', 'warm', 'draft',
   'Qualify renters and explain application requirements',
   '["Disclose AI assistant","Do not invent pricing","Transfer sensitive questions"]'::jsonb),
  ('bot_002', 'atrium', 'Owner Outreach', 'owner_outbound', 'en-US', 'confident', 'draft',
   'Explain property management service and schedule human follow-up',
   '["Respect opt-out","Use approved claims only"]'::jsonb)
ON CONFLICT (id) DO NOTHING;

INSERT INTO campaigns (id, tenant_id, bot_id, name, status, schedule, lead_count, max_attempts) VALUES
  ('camp_001', 'atrium', 'bot_001', 'Renter follow-up', 'scheduled', 'Weekdays 10:00-18:00', 32, 3),
  ('camp_002', 'atrium', 'bot_002', 'Owner warm leads', 'paused', 'Tue/Thu 11:00-16:00', 14, 2)
ON CONFLICT (id) DO NOTHING;

INSERT INTO calls (id, tenant_id, lead_id, campaign_id, lead_name, campaign_name, phone, status, outcome, duration_sec, started_at, summary) VALUES
  ('call_001', 'atrium', 'lead_001', 'camp_001', 'Maria Lopez', 'Renter follow-up', '+1 555 0101', 'completed', 'qualified', 286, now() - interval '2 hours',
   'Interested in Sunset Villas. Move-in next month. Needs pet policy confirmation.'),
  ('call_002', 'atrium', 'lead_003', 'camp_001', 'Ana Torres', 'Renter follow-up', '+1 555 0103', 'completed', 'callback', 91, now() - interval '1 hour',
   'Asked for callback after 17:00 with spouse present.'),
  ('call_003', 'atrium', 'lead_002', 'camp_002', 'Carlos Rivera', 'Owner warm leads', '+1 555 0102', 'queued', 'pending', 0, NULL, '')
ON CONFLICT (id) DO NOTHING;

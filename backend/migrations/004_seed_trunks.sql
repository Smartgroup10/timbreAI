-- Seed a sandbox trunk so the admin UI is not empty on first run.
-- The asterisk_endpoint MUST match the [name] of a PJSIP endpoint actually
-- registered in asterisk/etc/asterisk/pjsip.conf. The "internal" one points
-- to the local 6001 softphone for sandbox dialing.
INSERT INTO sip_trunks (id, name, provider, asterisk_endpoint, host, port, status, notes) VALUES
  ('trunk_internal', 'Internal sandbox (6001)', 'internal', '6001', '127.0.0.1', 5060, 'active',
   'Para pruebas locales. Registra un softphone (Zoiper, Linphone) como extension 6001 para responder.')
ON CONFLICT (id) DO NOTHING;

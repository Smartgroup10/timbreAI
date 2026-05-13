-- Asterisk PJSIP Realtime tables. Schema mínimo viable para outbound trunks
-- (endpoints + auths + aors + registrations). Asterisk descubre estas tablas
-- vía res_config_pgsql + extconfig.conf + sorcery.conf — ver asterisk/etc/asterisk/.
--
-- Asterisk 18+ comparte schema; los nombres de columnas y tipos son fijos. Si
-- añades columnas extra Asterisk las ignora.

-- AORs (Address Of Record): a dónde se contacta al endpoint.
CREATE TABLE IF NOT EXISTS ps_aors (
  id TEXT PRIMARY KEY,
  contact TEXT,
  max_contacts INTEGER,
  qualify_frequency INTEGER,
  authenticate_qualify TEXT,
  remove_existing TEXT,
  support_path TEXT
);

-- Auths: credenciales SIP.
CREATE TABLE IF NOT EXISTS ps_auths (
  id TEXT PRIMARY KEY,
  auth_type TEXT,
  username TEXT,
  password TEXT,
  realm TEXT,
  nonce_lifetime INTEGER
);

-- Endpoints: configuración del peer SIP. Apunta a un AOR (registro) y a un AUTH (credenciales).
CREATE TABLE IF NOT EXISTS ps_endpoints (
  id TEXT PRIMARY KEY,
  transport TEXT,
  aors TEXT,                       -- nombre del ps_aors.id asociado
  auth TEXT,                       -- inbound auth (cuando NOS llaman)
  outbound_auth TEXT,              -- outbound auth (cuando NOSOTROS llamamos)
  context TEXT,
  disallow TEXT,
  allow TEXT,
  direct_media TEXT,
  from_user TEXT,
  from_domain TEXT,
  callerid TEXT,
  dtmf_mode TEXT,
  force_rport TEXT,
  rewrite_contact TEXT,
  rtp_symmetric TEXT,
  send_pai TEXT,
  send_rpid TEXT,
  trust_id_inbound TEXT
);

-- Registrations: si el trunk requiere REGISTER (la mayoría de proveedores SIP).
CREATE TABLE IF NOT EXISTS ps_registrations (
  id TEXT PRIMARY KEY,
  outbound_auth TEXT,              -- nombre del ps_auths.id
  server_uri TEXT,                 -- sip:provider.example.com:5060
  client_uri TEXT,                 -- sip:user@provider.example.com
  retry_interval INTEGER,
  forbidden_retry_interval INTEGER,
  expiration INTEGER,
  transport TEXT,
  contact_user TEXT
);

-- Identify: matching por IP (sin REGISTER, p.ej. Twilio Elastic SIP Trunking).
CREATE TABLE IF NOT EXISTS ps_endpoint_id_ips (
  id TEXT PRIMARY KEY,
  endpoint TEXT,                   -- nombre del ps_endpoints.id
  match TEXT,                      -- IP o subnet del proveedor
  srv_lookups TEXT
);

-- Transports: normalmente se queda en pjsip.conf, pero la tabla DEBE existir
-- para que sorcery no se queje al arrancar.
CREATE TABLE IF NOT EXISTS ps_transports (
  id TEXT PRIMARY KEY,
  async_operations INTEGER,
  bind TEXT,
  ca_list_file TEXT,
  cert_file TEXT,
  cipher TEXT,
  domain TEXT,
  external_media_address TEXT,
  external_signaling_address TEXT,
  external_signaling_port INTEGER,
  method TEXT,
  local_net TEXT,
  password TEXT,
  priv_key_file TEXT,
  protocol TEXT,
  require_client_cert TEXT,
  verify_client TEXT,
  verify_server TEXT,
  tos TEXT,
  cos TEXT
);

-- Contacts: Asterisk los gestiona él mismo (registros dinámicos). Solo creamos la tabla.
CREATE TABLE IF NOT EXISTS ps_contacts (
  id TEXT PRIMARY KEY,
  uri TEXT,
  expiration_time BIGINT,
  qualify_frequency INTEGER,
  outbound_proxy TEXT,
  path TEXT,
  user_agent TEXT,
  qualify_timeout DOUBLE PRECISION,
  reg_server TEXT,
  authenticate_qualify TEXT,
  via_addr TEXT,
  via_port INTEGER,
  call_id TEXT,
  endpoint TEXT,
  prune_on_boot TEXT
);

-- ─────────────────────────────────────────────────────────────────────────
-- Metadata app-level: extendemos sip_trunks con los campos necesarios para
-- generar las filas correspondientes en ps_endpoints/ps_auths/ps_aors/ps_registrations.
-- ─────────────────────────────────────────────────────────────────────────
ALTER TABLE sip_trunks ADD COLUMN IF NOT EXISTS sip_username TEXT NOT NULL DEFAULT '';
ALTER TABLE sip_trunks ADD COLUMN IF NOT EXISTS sip_password TEXT NOT NULL DEFAULT '';
ALTER TABLE sip_trunks ADD COLUMN IF NOT EXISTS register_required BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE sip_trunks ADD COLUMN IF NOT EXISTS identify_ip TEXT NOT NULL DEFAULT '';

-- Borramos el seed antiguo (Internal sandbox apunta al 6001 que está en
-- pjsip.conf como endpoint estático — sigue funcionando, no necesita realtime).
-- El admin creará los trunks reales desde la UI.
DELETE FROM sip_trunks WHERE id = 'trunk_internal' AND sip_username = '';

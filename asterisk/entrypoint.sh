#!/bin/sh
# Genera todos los .conf en runtime. Patrón inspirado en Smartgroup10/SmartSIP.
# Realtime PJSIP vía ODBC (unixODBC + odbc-postgresql) en vez de res_config_pgsql
# nativo — ese último, en esta build de Asterisk, ignora silenciosamente nuestra
# config (parse_config no encuentra dbuser/dbpass aunque el .conf sea correcto).
set -e

echo "==> [entrypoint] Generando configs en /etc/asterisk/ y ODBC"

# ── ODBC: driver PostgreSQL ─────────────────────────────────────────────
# La ruta del .so varía según arquitectura: probamos las dos comunes y nos
# quedamos con la que exista. En Debian bookworm amd64 es x86_64-linux-gnu;
# en arm64 es aarch64-linux-gnu.
PSQLODBC=""
for path in /usr/lib/x86_64-linux-gnu/odbc/psqlodbcw.so \
            /usr/lib/aarch64-linux-gnu/odbc/psqlodbcw.so \
            /usr/lib/odbc/psqlodbcw.so; do
  [ -f "$path" ] && PSQLODBC="$path" && break
done
if [ -z "$PSQLODBC" ]; then
  echo "==> [entrypoint] ERROR: psqlodbcw.so no encontrado. Buscando…"
  find /usr -name 'psqlodbcw.so' 2>/dev/null || true
  exit 1
fi
echo "==> [entrypoint] driver PostgreSQL ODBC: $PSQLODBC"

cat > /etc/odbcinst.ini <<EOF
[PostgreSQL]
Description = PostgreSQL ODBC driver (Unicode)
Driver = $PSQLODBC
UsageCount = 1
EOF

# DSN "asterisk" — apunta al cluster Postgres compartido con el backend.
cat > /etc/odbc.ini <<EOF
[asterisk]
Description = Asterisk Realtime via ODBC
Driver = PostgreSQL
Servername = ${POSTGRES_HOST:-postgres}
Port = ${POSTGRES_PORT:-5432}
Database = ${POSTGRES_DB:-atrium_calls}
UserName = ${POSTGRES_USER:-atrium}
Password = ${POSTGRES_PASSWORD:-change-me}
Protocol = 9.6
ReadOnly = No
EOF

# ── res_odbc.conf: conexión ODBC desde Asterisk ─────────────────────────
cat > /etc/asterisk/res_odbc.conf <<EOF
[asterisk]
enabled = yes
dsn = asterisk
username = ${POSTGRES_USER:-atrium}
password = ${POSTGRES_PASSWORD:-change-me}
pre-connect = yes
sanitysql = SELECT 1
max_connections = 5
EOF

# ── ari.conf ────────────────────────────────────────────────────────────
cat > /etc/asterisk/ari.conf <<EOF
[general]
enabled = yes
pretty = yes
allowed_origins = *

[${ASTERISK_ARI_USER:-timbre}]
type = user
read_only = no
password = ${ASTERISK_ARI_PASSWORD:-change-me}
EOF

# ── http.conf ───────────────────────────────────────────────────────────
cat > /etc/asterisk/http.conf <<EOF
[general]
enabled = yes
bindaddr = 0.0.0.0
bindport = 8088
EOF

# ── extconfig.conf: mapping Realtime → ODBC ─────────────────────────────
cat > /etc/asterisk/extconfig.conf <<'EOF'
[settings]
ps_endpoints       => odbc,asterisk,ps_endpoints
ps_auths           => odbc,asterisk,ps_auths
ps_aors            => odbc,asterisk,ps_aors
ps_endpoint_id_ips => odbc,asterisk,ps_endpoint_id_ips
ps_registrations   => odbc,asterisk,ps_registrations
EOF

# ── sorcery.conf: realtime + config combinados ──────────────────────────
cat > /etc/asterisk/sorcery.conf <<'EOF'
[res_pjsip]
endpoint=realtime,ps_endpoints
endpoint=config,pjsip.conf,criteria=type=endpoint
auth=realtime,ps_auths
auth=config,pjsip.conf,criteria=type=auth
aor=realtime,ps_aors
aor=config,pjsip.conf,criteria=type=aor

[res_pjsip_endpoint_identifier_ip]
identify=realtime,ps_endpoint_id_ips

[res_pjsip_outbound_registration]
registration=realtime,ps_registrations
EOF

# ── pjsip.conf: global + transport + sandbox 6001 estático ──────────────
cat > /etc/asterisk/pjsip.conf <<'EOF'
[global]
type = global
user_agent = timbre.ai
max_initial_qualify_time = 0
keep_alive_interval = 90

[transport-udp]
type = transport
protocol = udp
bind = 0.0.0.0:5060

[6001]
type = endpoint
context = from-internal
disallow = all
allow = ulaw,alaw
auth = 6001-auth
aors = 6001-aor
direct_media = no
force_rport = yes
rewrite_contact = yes
rtp_symmetric = yes

[6001-auth]
type = auth
auth_type = userpass
username = 6001
password = change-me

[6001-aor]
type = aor
max_contacts = 1
remove_existing = yes
qualify_frequency = 30
EOF

# ── extensions.conf: dialplan mínimo ────────────────────────────────────
cat > /etc/asterisk/extensions.conf <<'EOF'
[general]
static = yes
writeprotect = no

[from-internal]
exten => 6001,1,Dial(PJSIP/6001,30,m)
 same => n,Hangup()

[from-trunk]
exten => _X.,1,NoOp(Inbound desde trunk SIP a ${EXTEN})
 same => n,Stasis(timbre-bot,inbound,${EXTEN})
 same => n,Hangup()
EOF

# ── rtp.conf ────────────────────────────────────────────────────────────
cat > /etc/asterisk/rtp.conf <<EOF
[general]
rtpstart = ${ASTERISK_RTP_START:-10000}
rtpend = ${ASTERISK_RTP_END:-10020}
strictrtp = yes
icesupport = no
EOF

# ── logger.conf ─────────────────────────────────────────────────────────
cat > /etc/asterisk/logger.conf <<'EOF'
[general]
dateformat = %F %T.%3q

[logfiles]
console => notice,warning,error
messages => notice,warning,error
EOF

# ── modules.conf ────────────────────────────────────────────────────────
# ODBC (res_odbc + res_config_odbc) preload obligatorio para que Sorcery
# encuentre el backend al inicializar. res_config_pgsql se queda fuera.
cat > /etc/asterisk/modules.conf <<'EOF'
[modules]
autoload = yes

preload = res_odbc.so
preload = res_config_odbc.so

noload = res_config_pgsql.so
noload = chan_sip.so
noload = chan_unistim.so
noload = chan_iax2.so
noload = chan_motif.so
noload = chan_console.so
noload = chan_alsa.so
noload = app_voicemail.so
noload = app_meetme.so
noload = app_jack.so
noload = app_getcpeid.so
noload = app_adsiprog.so
noload = res_adsi.so
noload = res_smdi.so
noload = res_fax.so
noload = res_fax_spandsp.so
noload = res_phoneprov.so
noload = res_pjsip_phoneprov_provider.so
noload = res_calendar.so
noload = res_calendar_ews.so
noload = res_calendar_caldav.so
noload = res_calendar_icalendar.so
noload = res_calendar_exchange.so
noload = res_xmpp.so
noload = res_snmp.so
noload = res_hep.so
noload = res_hep_pjsip.so
noload = res_hep_rtcp.so
noload = res_config_ldap.so
noload = cdr_pgsql.so
noload = cdr_tds.so
noload = cdr_radius.so
noload = cdr_sqlite3_custom.so
noload = cel_pgsql.so
noload = cel_tds.so
noload = cel_radius.so
noload = cel_sqlite3_custom.so
noload = pbx_lua.so
noload = pbx_ael.so
noload = pbx_dundi.so
noload = format_ogg_vorbis.so
EOF

# Permisos.
chown -R asterisk:asterisk /etc/asterisk /var/run/asterisk /var/log/asterisk /var/spool/asterisk 2>/dev/null || true

echo "==> [entrypoint] config rendered:"
echo "    DSN host=${POSTGRES_HOST:-postgres} db=${POSTGRES_DB:-atrium_calls} user=${POSTGRES_USER:-atrium}"

# Test ODBC opcional (no aborta si falla — Postgres puede no estar listo aún).
if command -v isql >/dev/null 2>&1; then
  echo "==> [entrypoint] probando DSN 'asterisk' con isql…"
  echo "SELECT 1;" | isql -v asterisk "${POSTGRES_USER:-atrium}" "${POSTGRES_PASSWORD:-change-me}" 2>&1 | sed 's/^/    /' | head -10 || \
    echo "    (test falló — puede ser que postgres aún no esté listo)"
fi

echo "==> [entrypoint] arrancando Asterisk..."
exec asterisk -f -U asterisk -G asterisk

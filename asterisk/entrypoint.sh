#!/bin/sh
# Genera TODOS los .conf en runtime con env vars. Patrón inspirado en
# Smartgroup10/SmartSIP — más robusto que envsubst + COPY porque:
#   - vemos por stdout exactamente qué se generó
#   - los permisos se aplican después (chown asterisk)
#   - una sola fuente de verdad (este script), no dos
set -e

echo "==> [entrypoint] Generando configs en /etc/asterisk/"

# ── res_config_pgsql.conf ────────────────────────────────────────────────
# Driver Realtime: lee endpoints/auths/aors/registrations/identifies de
# Postgres. Las credenciales son las del clúster compartido con el backend.
cat > /etc/asterisk/res_config_pgsql.conf <<EOF
[general]
dbhost=${POSTGRES_HOST:-postgres}
dbport=${POSTGRES_PORT:-5432}
dbname=${POSTGRES_DB:-atrium_calls}
dbuser=${POSTGRES_USER:-atrium}
dbpass=${POSTGRES_PASSWORD:-change-me}
appname=asterisk-realtime
requirements=warn
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

# ── extconfig.conf ──────────────────────────────────────────────────────
cat > /etc/asterisk/extconfig.conf <<'EOF'
[settings]
ps_endpoints       => pgsql,general,ps_endpoints
ps_auths           => pgsql,general,ps_auths
ps_aors            => pgsql,general,ps_aors
ps_endpoint_id_ips => pgsql,general,ps_endpoint_id_ips
ps_registrations   => pgsql,general,ps_registrations
EOF

# ── sorcery.conf ────────────────────────────────────────────────────────
# IMPORTANTE: combinamos realtime + config. Así los endpoints estáticos del
# pjsip.conf (sandbox 6001) coexisten con los realtime (trunks reales).
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

# ── pjsip.conf ──────────────────────────────────────────────────────────
# Solo global, transports y el softphone 6001 (sandbox). Los trunks reales
# viven en ps_* (BD) y los gestiona el portal admin.
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

# ── extensions.conf (dialplan mínimo) ───────────────────────────────────
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
# Autoload + noload de basura que ensucia logs (ALSA/jack), módulos legacy
# (chan_sip, app_meetme, AEL) y módulos PG/SQLite que no usamos pero que
# generan errores ruidosos en boot.
cat > /etc/asterisk/modules.conf <<'EOF'
[modules]
autoload = yes

; Preload res_config_pgsql para que Sorcery lo encuentre.
preload = res_config_pgsql.so

; Módulos que ensucian logs o fallan en el contenedor (sin tarjeta de
; sonido, sin libvorbisenc, sin SQLite custom...). Los apagamos.
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

# Permisos: si la imagen base creó /etc/asterisk con root:root estricto,
# Asterisk (que corre como usuario asterisk) no leería nuestros archivos
# y arrancaría con defaults silenciosamente. chown -R lo cura.
chown -R asterisk:asterisk /etc/asterisk /var/run/asterisk /var/log/asterisk /var/spool/asterisk 2>/dev/null || true

echo "==> [entrypoint] config rendered:"
echo "    dbhost=${POSTGRES_HOST:-postgres} dbname=${POSTGRES_DB:-atrium_calls} dbuser=${POSTGRES_USER:-atrium}"
echo "    ari_user=${ASTERISK_ARI_USER:-timbre}"
echo "==> [entrypoint] arrancando Asterisk..."

exec asterisk -f -U asterisk -G asterisk

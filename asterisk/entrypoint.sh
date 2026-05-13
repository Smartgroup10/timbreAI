#!/bin/sh
# Renderiza res_config_pgsql.conf con las credenciales del Postgres compartido
# (el backend y asterisk leen el MISMO cluster). Usamos heredoc shell puro en
# vez de envsubst para no depender de gettext (en el contenedor base puede no
# estar y silenciosamente abortábamos sin generar el archivo, dejando Asterisk
# con dbname=asterisk por defecto → "Failed to connect database asterisk on :").
set -e

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

echo "[entrypoint] res_config_pgsql.conf: dbhost=${POSTGRES_HOST:-postgres} dbname=${POSTGRES_DB:-atrium_calls} dbuser=${POSTGRES_USER:-atrium}"

# Pass-through al binario original. La imagen andrius/asterisk arranca con
# este mismo comando, así que solo añadimos el render previo.
exec asterisk -f -U asterisk -G asterisk

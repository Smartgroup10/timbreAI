; Configuración del driver Postgres para Asterisk Realtime.
; Este archivo es un TEMPLATE — el entrypoint corre envsubst y escribe
; res_config_pgsql.conf con los valores reales antes de arrancar Asterisk.

[general]
dbhost=postgres
dbport=5432
dbname=${POSTGRES_DB}
dbuser=${POSTGRES_USER}
dbpass=${POSTGRES_PASSWORD}
; appname identifica las conexiones en pg_stat_activity
appname=asterisk-realtime
requirements=warn

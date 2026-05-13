# Pre-built community image — base Debian, trae chan_pjsip, res_ari, res_odbc.
FROM andrius/asterisk:latest

# unixODBC + driver de Postgres. res_config_pgsql nativo no parsea bien la
# config en esta build de Asterisk (parse_config no encuentra dbuser/dbpass
# aunque el .conf esté bien). Vamos por ODBC, que es el camino que usa el
# repo Smartgroup10/SmartSIP en producción.
USER root
RUN apt-get update && apt-get install -y --no-install-recommends \
    unixodbc odbc-postgresql \
    && rm -rf /var/lib/apt/lists/*

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 5060/udp 8088 10000-10020/udp

ENTRYPOINT ["/entrypoint.sh"]

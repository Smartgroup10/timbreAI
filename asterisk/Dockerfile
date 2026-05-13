# Pre-built community image: trae chan_pjsip, res_ari, res_config_pgsql, core
# sounds y arranca como usuario `asterisk` sin trampas.
FROM andrius/asterisk:latest

# Toda la config la genera entrypoint.sh en runtime (patrón inspirado en
# Smartgroup10/SmartSIP). Eso permite interpolar env vars sin envsubst y nos
# da una única fuente de verdad de configs.
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 5060/udp 8088 10000-10020/udp

ENTRYPOINT ["/entrypoint.sh"]

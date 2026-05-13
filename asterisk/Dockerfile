# Pre-built community image: trae chan_pjsip, res_ari, res_config_pgsql, core
# sounds y arranca como usuario `asterisk` sin trampas.
FROM andrius/asterisk:latest

# envsubst (gettext) para renderizar *.conf.tpl con env vars (la BD password,
# etc.). La imagen base es Alpine — usamos apk; si en el futuro cambia a Debian
# habría que ajustar a apt-get install gettext-base.
USER root
RUN apk add --no-cache gettext || \
    (apt-get update && apt-get install -y --no-install-recommends gettext-base && rm -rf /var/lib/apt/lists/*)

# Configs estáticas — sobre los defaults de la imagen.
COPY etc/asterisk/ari.conf                /etc/asterisk/ari.conf
COPY etc/asterisk/http.conf               /etc/asterisk/http.conf
COPY etc/asterisk/extensions.conf         /etc/asterisk/extensions.conf
COPY etc/asterisk/pjsip.conf              /etc/asterisk/pjsip.conf
COPY etc/asterisk/rtp.conf                /etc/asterisk/rtp.conf
COPY etc/asterisk/extconfig.conf          /etc/asterisk/extconfig.conf
COPY etc/asterisk/sorcery.conf            /etc/asterisk/sorcery.conf
COPY etc/asterisk/res_config_pgsql.conf.tpl /etc/asterisk/res_config_pgsql.conf.tpl

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 5060/udp 8088 10000-10020/udp

ENTRYPOINT ["/entrypoint.sh"]

# Pre-built community image: trae chan_pjsip, res_ari, res_config_pgsql, core
# sounds y arranca como usuario `asterisk` sin trampas.
FROM andrius/asterisk:latest

# Configs estáticas — se copian sobre los defaults de la imagen.
# res_config_pgsql.conf NO se copia: se genera en runtime desde el entrypoint
# usando los env vars POSTGRES_* que comparte con el backend.
COPY etc/asterisk/ari.conf        /etc/asterisk/ari.conf
COPY etc/asterisk/http.conf       /etc/asterisk/http.conf
COPY etc/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY etc/asterisk/pjsip.conf      /etc/asterisk/pjsip.conf
COPY etc/asterisk/rtp.conf        /etc/asterisk/rtp.conf
COPY etc/asterisk/extconfig.conf  /etc/asterisk/extconfig.conf
COPY etc/asterisk/sorcery.conf    /etc/asterisk/sorcery.conf

COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

EXPOSE 5060/udp 8088 10000-10020/udp

ENTRYPOINT ["/entrypoint.sh"]

# Pre-built community image: trae chan_pjsip, res_ari, core sounds y arranca
# como usuario `asterisk` sin trampas. Pin del major tag para evitar sorpresas.
FROM andrius/asterisk:latest

# Copiamos nuestros configs SOBRE los defaults de la imagen.
# Evitamos bind-mounts en compose porque tener archivos individuales montados
# es frágil (Docker crea un dir vacío si el host file no existe en build dir,
# y rompe con "mount file onto directory").
COPY etc/asterisk/ari.conf        /etc/asterisk/ari.conf
COPY etc/asterisk/http.conf       /etc/asterisk/http.conf
COPY etc/asterisk/extensions.conf /etc/asterisk/extensions.conf
COPY etc/asterisk/pjsip.conf      /etc/asterisk/pjsip.conf
COPY etc/asterisk/rtp.conf        /etc/asterisk/rtp.conf

EXPOSE 5060/udp 8088 10000-10020/udp

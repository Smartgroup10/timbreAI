#!/bin/sh
# Renderiza cualquier *.conf.tpl bajo /etc/asterisk/ usando envsubst y luego
# arranca Asterisk. Esto nos da interpolación de env vars en configs (Asterisk
# no lo hace nativamente).
set -e

for tpl in /etc/asterisk/*.conf.tpl; do
  [ -e "$tpl" ] || continue
  out="${tpl%.tpl}"
  envsubst < "$tpl" > "$out"
  echo "[entrypoint] rendered $(basename "$out")"
done

# Comando original de la imagen base: asterisk en foreground.
exec asterisk -f -U asterisk -G asterisk

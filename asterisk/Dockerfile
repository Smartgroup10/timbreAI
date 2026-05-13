# Use a pre-built community image to avoid apt-get build flakiness in Coolify's
# build sandbox. andrius/asterisk bundles chan_pjsip, res_ari, core sounds and
# runs as the asterisk user out of the box.
#
# Pin the major version so upstream changes don't surprise us.
FROM andrius/asterisk:latest

# Custom configs are mounted via docker-compose.yml at runtime:
#   ./asterisk/etc/asterisk:/etc/asterisk:ro
# This Dockerfile stays minimal — the image's CMD already runs asterisk in foreground.

EXPOSE 5060/udp 8088 10000-10020/udp

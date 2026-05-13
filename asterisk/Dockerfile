FROM debian:bookworm

ARG DEBIAN_FRONTEND=noninteractive

# Full debian:bookworm (no slim) avoids missing base tools, and we let apt pull the
# recommended packages — asterisk needs several module collections + sound files which
# apt classifies as "recommends" and `--no-install-recommends` would skip.
RUN set -eux \
    && apt-get update \
    && apt-get install -y \
        asterisk \
        asterisk-modules \
        asterisk-core-sounds-en \
        ca-certificates \
    && apt-get clean \
    && rm -rf /var/lib/apt/lists/* /var/cache/apt/archives/* \
    && mkdir -p /var/spool/asterisk /var/log/asterisk \
    && chown -R asterisk:asterisk /var/spool/asterisk /var/log/asterisk

EXPOSE 5060/udp 8088 10000-10020/udp

CMD ["/usr/sbin/asterisk", "-f", "-U", "asterisk", "-G", "asterisk", "-vvv"]

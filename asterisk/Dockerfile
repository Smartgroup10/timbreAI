FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends asterisk ca-certificates \
    && rm -rf /var/lib/apt/lists/*

EXPOSE 5060/udp 8088 10000-10020/udp

CMD ["asterisk", "-f", "-U", "asterisk", "-G", "asterisk"]


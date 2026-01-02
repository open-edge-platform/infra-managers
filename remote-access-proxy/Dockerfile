FROM alpine:3.20

RUN apk add --no-cache \
      ca-certificates \
      tzdata \
      curl \
      busybox-extras \
      iproute2 \
      openssh-client \
  && adduser -D -H -u 10001 appuser

WORKDIR /app

COPY remote-access-proxy /app/remote-access-proxy
RUN chmod +x /app/remote-access-proxy && ls -l /app

# 8080 – Endpoint proxy (for Chisel server)
# 8000 –  reverse-SSH
EXPOSE 8080 8000

USER appuser

ENTRYPOINT ["/app/remote-access-proxy"]
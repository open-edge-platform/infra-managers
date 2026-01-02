FROM alpine:3.20

WORKDIR /app

COPY proxy-manager /app/proxy-manager
RUN adduser -D -u 1000 appuser
USER appuser
ENTRYPOINT ["/app/proxy-manager"]
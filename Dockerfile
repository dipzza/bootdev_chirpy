FROM debian:stable-slim

COPY bootdev_chirpy /bootdev_chirpy
COPY .env /.env

ENV PORT=8080

CMD ["/bootdev_chirpy"]
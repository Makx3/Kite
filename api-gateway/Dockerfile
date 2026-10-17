# ─ Etapa de build ──────────────────────────────────────────
FROM golang:1.27-alpine AS build

WORKDIR /src

# Copiar solo los manifiestos primero para aprovechar la caché de
# capas: cambian mucho menos que el código.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/api-gateway ./cmd/server

# ─ Etapa de runtime ────────────────────────────────────────
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 kite

USER kite
COPY --from=build /out/api-gateway /usr/local/bin/api-gateway

# El puerto expuesto es solo documentación: el valor real lo
# define GATEWAY_HTTP_PORT en el entorno (compose/.env).
EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/api-gateway"]

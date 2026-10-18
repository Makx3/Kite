# ─ Etapa de build ──────────────────────────────────────────
FROM golang:1.27-alpine AS build

WORKDIR /src

# Copiar solo los manifiestos primero para aprovechar la caché de
# capas: cambian mucho menos que el código.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/likes-svc ./cmd/server

# ─ Etapa de runtime ────────────────────────────────────────
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 kite

USER kite
COPY --from=build /out/likes-svc /usr/local/bin/likes-svc

# El puerto expuesto es solo documentación: el valor real lo
# define LIKES_GRPC_PORT en el entorno (compose/.env).
EXPOSE 50056

ENTRYPOINT ["/usr/local/bin/likes-svc"]

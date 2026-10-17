# ─ Etapa de build ──────────────────────────────────────────
FROM golang:1.27-alpine AS build

WORKDIR /src

# Copiar solo los manifiestos primero para aprovechar la caché de
# capas: cambian mucho menos que el código.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/users-svc ./cmd/server

# ─ Etapa de runtime ────────────────────────────────────────
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 kite

USER kite
COPY --from=build /out/users-svc /usr/local/bin/users-svc

# El puerto expuesto es solo documentación: el valor real lo
# define USERS_GRPC_PORT en el entorno (compose/.env).
EXPOSE 50051

ENTRYPOINT ["/usr/local/bin/users-svc"]

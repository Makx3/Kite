FROM golang:1.27-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags='-s -w' -o /out/posts-svc ./cmd/server
FROM alpine:3.22
RUN apk add --no-cache ca-certificates && adduser -D -u 10001 kite
USER kite
COPY --from=build /out/posts-svc /usr/local/bin/posts-svc
EXPOSE 50054
ENTRYPOINT ["/usr/local/bin/posts-svc"]

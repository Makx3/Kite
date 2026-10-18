// Package config carga toda la configuración del servicio Posts desde
// variables de entorno. No hay valores hardcodeados en el código: el
// servicio falla al arrancar si falta algún valor obligatorio.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config es la configuración efectiva del servicio, ya validada.
type Config struct {
	// GRPCPort es el puerto TCP escuchado por el servidor gRPC.
	GRPCPort int
	// DBDSN es la URI de conexión a MongoDB (posts_db).
	DBDSN string
	// FeedAddr es la dirección gRPC del servicio Feed (host:puerto).
	FeedAddr string
	// RPCTimeout es el tiempo máximo por invocación gRPC entrante.
	RPCTimeout time.Duration
	// ShutdownTimeout es el tiempo máximo para drenar peticiones
	// activas durante el apagado ordenado.
	ShutdownTimeout time.Duration
	// DBConnectRetries define cuántas veces reintentar la conexión a
	// MongoDB antes de desistir.
	DBConnectRetries int
	// FeedTimeout es el tiempo máximo para llamar a Feed.RefreshFeed.
	FeedTimeout time.Duration
}

// FromEnv lee y valida la configuración desde el entorno.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("POSTS_GRPC_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: POSTS_GRPC_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: POSTS_GRPC_PORT inválida: %q", portStr)
	}
	cfg.GRPCPort = port

	cfg.DBDSN = os.Getenv("POSTS_DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("config: POSTS_DB_DSN es obligatoria")
	}

	cfg.FeedAddr = os.Getenv("POSTS_FEED_ADDR")
	if cfg.FeedAddr == "" {
		return nil, fmt.Errorf("config: POSTS_FEED_ADDR es obligatoria")
	}

	if cfg.RPCTimeout, err = durationEnv("POSTS_RPC_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("POSTS_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.FeedTimeout, err = durationEnv("POSTS_FEED_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}

	retriesStr := os.Getenv("POSTS_DB_CONNECT_RETRIES")
	cfg.DBConnectRetries = 10
	if retriesStr != "" {
		n, err := strconv.Atoi(retriesStr)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("config: POSTS_DB_CONNECT_RETRIES inválida: %q", retriesStr)
		}
		cfg.DBConnectRetries = n
	}

	return cfg, nil
}

func durationEnv(name string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("config: %s inválida: %q (se espera algo como \"5s\")", name, raw)
	}
	return d, nil
}

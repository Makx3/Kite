// Package config carga toda la configuración del servicio Feed desde
// variables de entorno. Ninguna URL, puerto o credencial vive en el
// código: el servicio falla al arrancar si falta un valor obligatorio.
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
	// DBDSN es la URL de conexión a Redis (feed_db).
	DBDSN string
	// RelationshipsAddr es la dirección gRPC (host:puerto) de
	// Relationships, la fuente de verdad del grafo de seguidores.
	RelationshipsAddr string
	// RelationshipsTimeout acota cada llamada a Relationships para que
	// un servicio lento no arrastre la latencia de Feed.
	RelationshipsTimeout time.Duration
	// RPCTimeout es el tiempo máximo por invocación gRPC.
	RPCTimeout time.Duration
	// ShutdownTimeout es el tiempo máximo para drenar peticiones
	// activas durante el apagado ordenado.
	ShutdownTimeout time.Duration
	// DBConnectRetries define cuántas veces reintentar la conexión a
	// Redis antes de desistir.
	DBConnectRetries int
	// TTL es la vigencia de cada muro; 0 significa sin expiración.
	TTL time.Duration
}

// FromEnv lee y valida la configuración desde el entorno.
// FEED_GRPC_PORT, FEED_DB_DSN y FEED_RELATIONSHIPS_ADDR son
// obligatorias; los tiempos y la vigencia tienen valores por defecto.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("FEED_GRPC_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: FEED_GRPC_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: FEED_GRPC_PORT inválida: %q", portStr)
	}
	cfg.GRPCPort = port

	cfg.DBDSN = os.Getenv("FEED_DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("config: FEED_DB_DSN es obligatoria")
	}

	cfg.RelationshipsAddr = os.Getenv("FEED_RELATIONSHIPS_ADDR")
	if cfg.RelationshipsAddr == "" {
		return nil, fmt.Errorf("config: FEED_RELATIONSHIPS_ADDR es obligatoria")
	}

	if cfg.RPCTimeout, err = durationEnv("FEED_RPC_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("FEED_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.RelationshipsTimeout, err = durationEnv("FEED_RELATIONSHIPS_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}

	if cfg.TTL, err = ttlEnv("FEED_TTL_SECONDS"); err != nil {
		return nil, err
	}

	cfg.DBConnectRetries = 10
	if raw := os.Getenv("FEED_DB_CONNECT_RETRIES"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("config: FEED_DB_CONNECT_RETRIES inválida: %q", raw)
		}
		cfg.DBConnectRetries = n
	}

	return cfg, nil
}

// durationEnv lee una duración Go (ej. "5s") con valor por defecto.
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

// ttlEnv lee una cantidad de segundos. 0 (o el ausente) deshabilita la
// expiración del muro.
func ttlEnv(name string) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("config: %s inválida: %q (segundos enteros >= 0)", name, raw)
	}
	return time.Duration(n) * time.Second, nil
}

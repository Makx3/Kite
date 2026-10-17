// Package config carga toda la configuración del servicio desde
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
	// DBDSN es el connection string de PostgreSQL (users_db).
	DBDSN string
	// RPCTimeout es el tiempo máximo por invocación gRPC.
	RPCTimeout time.Duration
	// ShutdownTimeout es el tiempo máximo para drenar peticiones
	// activas durante el apagado ordenado.
	ShutdownTimeout time.Duration
	// DBMaxConns limita el pool de conexiones a la base de datos.
	DBMaxConns int32
	// DBHealthInterval define cada cuánto se pinguea la base de datos
	// para actualizar el estado del health check gRPC.
	DBHealthInterval time.Duration
}

// FromEnv lee y valida la configuración desde el entorno.
// USUARIOS_GRPC_PORT y USUARIOS_DB_DSN son obligatorios; los tiempos
// y el pool tienen valores por defecto razonables.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("USERS_GRPC_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: USERS_GRPC_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: USERS_GRPC_PORT inválida: %q", portStr)
	}
	cfg.GRPCPort = port

	cfg.DBDSN = os.Getenv("USERS_DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("config: USERS_DB_DSN es obligatoria")
	}

	if cfg.RPCTimeout, err = durationEnv("USERS_RPC_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("USERS_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.DBHealthInterval, err = durationEnv("USERS_DB_HEALTH_INTERVAL", 10*time.Second); err != nil {
		return nil, err
	}

	maxConnsStr := os.Getenv("USERS_DB_MAX_CONNS")
	cfg.DBMaxConns = 10
	if maxConnsStr != "" {
		n, err := strconv.Atoi(maxConnsStr)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("config: USERS_DB_MAX_CONNS inválida: %q", maxConnsStr)
		}
		cfg.DBMaxConns = int32(n)
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

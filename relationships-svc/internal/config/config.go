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
	// DBDSN es el connection string de PostgreSQL (relationships_db).
	DBDSN string
	// NotificationsAddr es la dirección gRPC (host:puerto) del servicio
	// de Notificaciones, al que se envían las alertas de nuevo seguidor.
	NotificationsAddr string
	// NotificationsTimeout acota cada llamada a Notifications para que un
	// servicio lento no arrastre la latencia de Relationships.
	NotificationsTimeout time.Duration
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
// RELATIONSHIPS_GRPC_PORT, RELATIONSHIPS_DB_DSN y
// RELATIONSHIPS_NOTIFICATIONS_ADDR son obligatorios.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("RELATIONSHIPS_GRPC_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: RELATIONSHIPS_GRPC_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: RELATIONSHIPS_GRPC_PORT inválida: %q", portStr)
	}
	cfg.GRPCPort = port

	cfg.DBDSN = os.Getenv("RELATIONSHIPS_DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("config: RELATIONSHIPS_DB_DSN es obligatoria")
	}

	cfg.NotificationsAddr = os.Getenv("RELATIONSHIPS_NOTIFICATIONS_ADDR")
	if cfg.NotificationsAddr == "" {
		return nil, fmt.Errorf("config: RELATIONSHIPS_NOTIFICATIONS_ADDR es obligatoria")
	}

	if cfg.RPCTimeout, err = durationEnv("RELATIONSHIPS_RPC_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("RELATIONSHIPS_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.DBHealthInterval, err = durationEnv("RELATIONSHIPS_DB_HEALTH_INTERVAL", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.NotificationsTimeout, err = durationEnv("RELATIONSHIPS_NOTIFICATIONS_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}

	cfg.DBMaxConns = 10
	if raw := os.Getenv("RELATIONSHIPS_DB_MAX_CONNS"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("config: RELATIONSHIPS_DB_MAX_CONNS inválida: %q", raw)
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

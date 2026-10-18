// Package config carga toda la configuración del servicio Likes desde
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
	// DBDSN es la URI de conexión a MongoDB (likes_db).
	DBDSN string
	// NotificationsAddr es la dirección gRPC (host:puerto) del
	// servicio de Notificaciones, al que se envía el alerta de like.
	NotificationsAddr string
	// NotificationsTimeout acota cada llamada a Notifications para
	// que un servicio lento no arrastre la latencia de Likes.
	NotificationsTimeout time.Duration
	// RPCTimeout es el tiempo máximo por invocación gRPC.
	RPCTimeout time.Duration
	// ShutdownTimeout es el tiempo máximo para drenar peticiones
	// activas durante el apagado ordenado.
	ShutdownTimeout time.Duration
	// DBConnectRetries define cuántas veces reintentar la conexión a
	// MongoDB antes de desistir.
	DBConnectRetries int
}

// FromEnv lee y valida la configuración desde el entorno.
// LIKES_GRPC_PORT, LIKES_DB_DSN y LIKES_NOTIFICATIONS_ADDR son
// obligatorias; los tiempos tienen valores por defecto.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("LIKES_GRPC_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: LIKES_GRPC_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: LIKES_GRPC_PORT inválida: %q", portStr)
	}
	cfg.GRPCPort = port

	cfg.DBDSN = os.Getenv("LIKES_DB_DSN")
	if cfg.DBDSN == "" {
		return nil, fmt.Errorf("config: LIKES_DB_DSN es obligatoria")
	}

	cfg.NotificationsAddr = os.Getenv("LIKES_NOTIFICATIONS_ADDR")
	if cfg.NotificationsAddr == "" {
		return nil, fmt.Errorf("config: LIKES_NOTIFICATIONS_ADDR es obligatoria")
	}

	if cfg.RPCTimeout, err = durationEnv("LIKES_RPC_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("LIKES_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}
	if cfg.NotificationsTimeout, err = durationEnv("LIKES_NOTIFICATIONS_TIMEOUT", 2*time.Second); err != nil {
		return nil, err
	}

	cfg.DBConnectRetries = 10
	if retriesStr := os.Getenv("LIKES_DB_CONNECT_RETRIES"); retriesStr != "" {
		n, err := strconv.Atoi(retriesStr)
		if err != nil || n < 1 {
			return nil, fmt.Errorf("config: LIKES_DB_CONNECT_RETRIES inválida: %q", retriesStr)
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

// Package config carga toda la configuración del API Gateway desde
// variables de entorno. Ninguna dirección, puerto o tiempo vive en el
// código: el servicio falla al arrancar si falta un valor obligatorio.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config es la configuración efectiva del gateway, ya validada.
type Config struct {
	// HTTPPort es el puerto TCP donde se expone la API REST. Es la
	// única superficie externa del sistema.
	HTTPPort int
	// UsersAddr es la dirección gRPC (host:puerto) del servicio Users.
	UsersAddr string
	// RelationshipsAddr es la dirección gRPC del servicio Relationships.
	RelationshipsAddr string
	// NotificationsAddr es la dirección gRPC del servicio Notifications.
	NotificationsAddr string
	// BackendTimeout acota cada llamada gRPC saliente, para que un
	// servicio lento no arrastre la latencia del gateway.
	BackendTimeout time.Duration
	// HandlerTimeout acota la petición HTTP completa (un handler puede
	// encadenar varias llamadas gRPC). Es la red de seguridad del gateway.
	HandlerTimeout time.Duration
	// ShutdownTimeout es el tiempo máximo para drenar peticiones activas
	// durante el apagado ordenado.
	ShutdownTimeout time.Duration
}

// FromEnv lee y valida la configuración desde el entorno. Son
// obligatorias: GATEWAY_HTTP_PORT, GATEWAY_USERS_ADDR,
// GATEWAY_RELATIONSHIPS_ADDR y GATEWAY_NOTIFICATIONS_ADDR.
func FromEnv() (*Config, error) {
	cfg := &Config{}

	portStr := os.Getenv("GATEWAY_HTTP_PORT")
	if portStr == "" {
		return nil, fmt.Errorf("config: GATEWAY_HTTP_PORT es obligatoria")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, fmt.Errorf("config: GATEWAY_HTTP_PORT inválida: %q", portStr)
	}
	cfg.HTTPPort = port

	// Se validan en orden fijo (no en un mapa) para que el mensaje de
	// error sea reproducible entre ejecuciones.
	if cfg.UsersAddr = os.Getenv("GATEWAY_USERS_ADDR"); cfg.UsersAddr == "" {
		return nil, fmt.Errorf("config: GATEWAY_USERS_ADDR es obligatoria")
	}
	if cfg.RelationshipsAddr = os.Getenv("GATEWAY_RELATIONSHIPS_ADDR"); cfg.RelationshipsAddr == "" {
		return nil, fmt.Errorf("config: GATEWAY_RELATIONSHIPS_ADDR es obligatoria")
	}
	if cfg.NotificationsAddr = os.Getenv("GATEWAY_NOTIFICATIONS_ADDR"); cfg.NotificationsAddr == "" {
		return nil, fmt.Errorf("config: GATEWAY_NOTIFICATIONS_ADDR es obligatoria")
	}

	if cfg.BackendTimeout, err = durationEnv("GATEWAY_BACKEND_TIMEOUT", 3*time.Second); err != nil {
		return nil, err
	}
	if cfg.HandlerTimeout, err = durationEnv("GATEWAY_HANDLER_TIMEOUT", 5*time.Second); err != nil {
		return nil, err
	}
	if cfg.ShutdownTimeout, err = durationEnv("GATEWAY_SHUTDOWN_TIMEOUT", 10*time.Second); err != nil {
		return nil, err
	}

	return cfg, nil
}

// durationEnv lee una duración Go (ej. "3s") con valor por defecto.
func durationEnv(name string, def time.Duration) (time.Duration, error) {
	raw := os.Getenv(name)
	if raw == "" {
		return def, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil || d <= 0 {
		return 0, fmt.Errorf("config: %s inválida: %q (se espera algo como \"3s\")", name, raw)
	}
	return d, nil
}

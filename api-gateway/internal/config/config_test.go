package config

import (
	"strings"
	"testing"
	"time"
)

// setBase define las cuatro variables obligatorias; cada prueba quita o
// cambia la que le interesa.
func setBase(t *testing.T) {
	t.Helper()
	t.Setenv("GATEWAY_HTTP_PORT", "8080")
	t.Setenv("GATEWAY_USERS_ADDR", "users-svc:50051")
	t.Setenv("GATEWAY_RELATIONSHIPS_ADDR", "relationships-svc:50052")
	t.Setenv("GATEWAY_NOTIFICATIONS_ADDR", "notifications-svc:50053")
}

func TestFromEnvCompleta(t *testing.T) {
	setBase(t)
	t.Setenv("GATEWAY_BACKEND_TIMEOUT", "2s")

	cfg, err := FromEnv()
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	if cfg.HTTPPort != 8080 {
		t.Errorf("puerto inesperado: %d", cfg.HTTPPort)
	}
	if cfg.UsersAddr != "users-svc:50051" || cfg.NotificationsAddr != "notifications-svc:50053" {
		t.Errorf("direcciones mal leídas: %+v", cfg)
	}
	if cfg.BackendTimeout != 2*time.Second {
		t.Errorf("timeout inesperado: %v", cfg.BackendTimeout)
	}
	// Los opcionales toman su valor por defecto.
	if cfg.ShutdownTimeout != 10*time.Second || cfg.HandlerTimeout != 5*time.Second {
		t.Errorf("valores por defecto inesperados: %+v", cfg)
	}
}

func TestFromEnvObligatorias(t *testing.T) {
	casos := []struct {
		nombre string
		quitar string
		quiere string
	}{
		{"sin puerto", "GATEWAY_HTTP_PORT", "GATEWAY_HTTP_PORT"},
		{"sin users", "GATEWAY_USERS_ADDR", "GATEWAY_USERS_ADDR"},
		{"sin relationships", "GATEWAY_RELATIONSHIPS_ADDR", "GATEWAY_RELATIONSHIPS_ADDR"},
		{"sin notifications", "GATEWAY_NOTIFICATIONS_ADDR", "GATEWAY_NOTIFICATIONS_ADDR"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			setBase(t)
			t.Setenv(c.quitar, "")

			_, err := FromEnv()
			if err == nil {
				t.Fatalf("esperaba error por %s ausente", c.quitar)
			}
			if !strings.Contains(err.Error(), c.quiere) {
				t.Errorf("el error debe nombrar %s, fue: %v", c.quiere, err)
			}
		})
	}
}

func TestFromEnvPuertoInvalido(t *testing.T) {
	setBase(t)
	for _, raw := range []string{"no-numero", "0", "70000"} {
		t.Run(raw, func(t *testing.T) {
			t.Setenv("GATEWAY_HTTP_PORT", raw)
			if _, err := FromEnv(); err == nil {
				t.Errorf("esperaba error con puerto %q", raw)
			}
		})
	}
}

func TestFromEnvDuracionInvalida(t *testing.T) {
	setBase(t)
	t.Setenv("GATEWAY_BACKEND_TIMEOUT", "3 segundos")
	if _, err := FromEnv(); err == nil {
		t.Error("esperaba error con duración inválida")
	}
}

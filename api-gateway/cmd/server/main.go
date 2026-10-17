// Command server levanta el API Gateway: la única superficie HTTP del
// sistema. Recibe REST, delega por gRPC a los servicios de negocio y agrega
// respuestas cuando el caso de uso lo exige. Arranca aunque un servicio esté
// caído (las conexiones gRPC son perezosas) y devuelve 503 en ese caso en
// lugar de caerse.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Makx3/Kite/api-gateway/internal/clients"
	"github.com/Makx3/Kite/api-gateway/internal/config"
	"github.com/Makx3/Kite/api-gateway/internal/httpapi"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("servicio terminó con error", "error", err)
		os.Exit(1)
	}
	logger.Info("servicio detenido ordenadamente")
}

func run(logger *slog.Logger) error {
	cfg, err := config.FromEnv()
	if err != nil {
		return err
	}

	// ctx cancelable controla el ciclo de vida: SIGINT/SIGTERM lo cancela.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Clientes hacia los tres servicios de negocio. Al ser conexiones
	// perezosas, el gateway arranca aunque alguno esté caído: el fallo se
	// degrada por petición (503) sin tumbar el proceso.
	users, err := clients.NewUsers(cfg.UsersAddr, cfg.BackendTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = users.Close() }()

	relationships, err := clients.NewRelationships(cfg.RelationshipsAddr, cfg.BackendTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = relationships.Close() }()

	notifications, err := clients.NewNotifications(cfg.NotificationsAddr, cfg.BackendTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = notifications.Close() }()

	logger.Info("clientes gRPC configurados",
		"users", cfg.UsersAddr,
		"relationships", cfg.RelationshipsAddr,
		"notifications", cfg.NotificationsAddr,
	)

	handler := httpapi.NewRouter(httpapi.Deps{
		Users:          users,
		Relationships:  relationships,
		Notifications:  notifications,
		HandlerTimeout: cfg.HandlerTimeout,
	}, logger)

	srv := &http.Server{
		Addr:              fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.ListenAndServe() }()
	logger.Info("API Gateway escuchando", "puerto", cfg.HTTPPort)

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("servidor HTTP: %w", err)
		}
		return nil
	case <-ctx.Done():
	}

	logger.Info("apagando: drenando peticiones activas")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("drenado excedió el tiempo límite, forzando parada", "error", err)
		_ = srv.Close()
	}

	if err := <-serveErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("servidor HTTP: %w", err)
	}
	return nil
}

// Command server levanta el microservicio Likes (reacciones a
// publicaciones): servidor gRPC con reintentos de arranque contra
// MongoDB, health check periódico, disparador hacia Notifications y
// apagado ordenado.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/Makx3/Kite/likes-svc/internal/config"
	"github.com/Makx3/Kite/likes-svc/internal/handler"
	"github.com/Makx3/Kite/likes-svc/internal/notify"
	"github.com/Makx3/Kite/likes-svc/internal/server"
	"github.com/Makx3/Kite/likes-svc/internal/service"
	"github.com/Makx3/Kite/likes-svc/internal/store"
	pb "github.com/Makx3/Kite/likes-svc/pb"
)

// retryBackoff calcula un backoff exponencial con tope, para no
// martillar la BD si tarda en estar disponible.
func retryBackoff(attempt int) time.Duration {
	const (
		base = 500 * time.Millisecond
		max  = 5 * time.Second
	)
	d := base << attempt
	if d > max {
		return max
	}
	return d
}

// connectWithRetry intenta conectar y migrar la base hasta que
// funcione o se agote el contexto (apagado/timeout externo). Así el
// servicio espera a MongoDB en arranques simultáneos (compose) sin
// morir ni requerir depends_on frágiles.
func connectWithRetry(ctx context.Context, logger *slog.Logger, dsn string, maxAttempts int) (*store.MongoDB, error) {
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoff(attempt)):
			}
		}
		m, err := store.NewMongo(ctx, dsn)
		if err == nil {
			if err := m.Migrate(ctx); err != nil {
				_ = m.Close()
				logger.Warn("migración falló, reintentando", "intento", attempt+1, "error", err)
				continue
			}
			return m, nil
		}
		logger.Warn("base de datos no disponible, reintentando", "intento", attempt+1, "error", err)
	}
	return nil, fmt.Errorf("store: sin conexión tras %d intentos", maxAttempts)
}

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

	// ctx cancelable controla el ciclo de vida: SIGINT/SIGTERM lo
	// cancela y todo lo que lo respeta se detiene.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	m, err := connectWithRetry(ctx, logger, cfg.DBDSN, cfg.DBConnectRetries)
	if err != nil {
		return err
	}
	defer func() { _ = m.Close() }()
	logger.Info("conectado a MongoDB y migraciones aplicadas")

	// Cliente hacia Notifications. La conexión es perezosa, así que el
	// servicio arranca aunque Notifications no esté disponible; el fallo
	// se degrada en la capa de servicio sin romper la reacción.
	notifier, err := notify.New(cfg.NotificationsAddr, cfg.NotificationsTimeout)
	if err != nil {
		return err
	}
	defer func() { _ = notifier.Close() }()
	logger.Info("cliente de Notifications configurado", "addr", cfg.NotificationsAddr)

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Ping periódico: si la BD cae, el health check pasa a NOT_SERVING;
	// el servicio sigue vivo y responde errores ordenados.
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err := m.Ping(pingCtx)
				cancel()
				state := healthpb.HealthCheckResponse_SERVING
				if err != nil {
					state = healthpb.HealthCheckResponse_NOT_SERVING
					logger.Warn("health check: base de datos no responde", "error", err)
				}
				healthSrv.SetServingStatus("", state)
			}
		}
	}()

	grpcSrv := grpc.NewServer(server.UnaryInterceptors(cfg.RPCTimeout, logger))
	pb.RegisterLikeServiceServer(grpcSrv, handler.NewLikeServer(service.New(m, notifier, logger)))
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	reflection.Register(grpcSrv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en :%d: %w", cfg.GRPCPort, err)
	}
	logger.Info("servidor gRPC escuchando", "puerto", cfg.GRPCPort)

	serveErr := make(chan error, 1)
	go func() { serveErr <- grpcSrv.Serve(lis) }()

	<-ctx.Done()
	logger.Info("apagando: drenando peticiones activas")

	stopped := make(chan struct{})
	go func() {
		grpcSrv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(cfg.ShutdownTimeout):
		logger.Warn("drenado excedió el tiempo límite, forzando parada")
		grpcSrv.Stop()
	}

	if err := <-serveErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

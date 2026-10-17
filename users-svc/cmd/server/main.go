// Command server levanta el microservicio de Usuarios: servidor
// gRPC con reintentos de arranque contra PostgreSQL, health check
// periódico y apagado ordenado.
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

	"github.com/Makx3/Kite/users-svc/internal/config"
	"github.com/Makx3/Kite/users-svc/internal/handler"
	"github.com/Makx3/Kite/users-svc/internal/server"
	"github.com/Makx3/Kite/users-svc/internal/service"
	"github.com/Makx3/Kite/users-svc/internal/store"
	pb "github.com/Makx3/Kite/users-svc/pb"
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
// servicio espera a PostgreSQL en arranques simultáneos (compose)
// sin morir ni requerir depends_on frágiles.
func connectWithRetry(ctx context.Context, logger *slog.Logger, dsn string, maxConns int32) (*store.Postgres, error) {
	const maxAttempts = 10
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoff(attempt)):
			}
		}
		pg, err := store.NewPostgres(ctx, dsn, maxConns)
		if err == nil {
			if err := pg.Migrate(ctx); err != nil {
				pg.Close()
				logger.Warn("migración falló, reintentando", "intento", attempt+1, "error", err)
				continue
			}
			return pg, nil
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

	// ctx cancelable controla el ciclo de vida: la señal SIGINT/
	// SIGTERM lo cancela y todo lo que lo respeta se detiene.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pg, err := connectWithRetry(ctx, logger, cfg.DBDSN, cfg.DBMaxConns)
	if err != nil {
		return err
	}
	defer pg.Close()
	logger.Info("conectado a PostgreSQL y migraciones aplicadas")

	healthSrv := health.NewServer()
	// Estado inicial: sirviendo (la BD acaba de responder en el ping).
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

	// Ping periódico: si la BD cae, el health check pasa a NOT_SERVING
	// para que el orquestador/routing se entere; el servicio sigue vivo
	// y responde errores ordenados a los RPC afectados.
	go func() {
		ticker := time.NewTicker(cfg.DBHealthInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				pingCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				err := pg.Ping(pingCtx)
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
	pb.RegisterUserServiceServer(grpcSrv, handler.NewUserServer(service.New(pg)))
	healthpb.RegisterHealthServer(grpcSrv, healthSrv)
	reflection.Register(grpcSrv) // permite grpcurl sin protos en el cliente

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en :%d: %w", cfg.GRPCPort, err)
	}
	logger.Info("servidor gRPC escuchando", "puerto", cfg.GRPCPort)

	// Bloquea hasta la señal; al cancelarse ctx, se drena lo activo.
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

	// Serve devuelve grpc.ErrServerStopped al parar; no es un fallo.
	if err := <-serveErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

// Command server levanta el microservicio Posts: servidor gRPC con
// reintentos de arranque contra MongoDB, health check periódico y
// apagado ordenado. Al crear un post, notifica al servicio Feed.
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

	"github.com/Makx3/Kite/posts-svc/internal/config"
	"github.com/Makx3/Kite/posts-svc/internal/handler"
	"github.com/Makx3/Kite/posts-svc/internal/notify"
	"github.com/Makx3/Kite/posts-svc/internal/server"
	"github.com/Makx3/Kite/posts-svc/internal/service"
	"github.com/Makx3/Kite/posts-svc/internal/store"
	pb "github.com/Makx3/Kite/posts-svc/pb"
)

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

func connectWithRetry(ctx context.Context, logger *slog.Logger, dsn string, retries int) (*store.MongoDB, error) {
	for attempt := 0; attempt < retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(retryBackoff(attempt)):
			}
		}
		m, err := store.NewPostgres(ctx, dsn, 0)
		if err == nil {
			if err := m.Migrate(ctx); err != nil {
				m.Close()
				logger.Warn("migración falló, reintentando", "intento", attempt+1, "error", err)
				continue
			}
			return m, nil
		}
		logger.Warn("base de datos no disponible, reintentando", "intento", attempt+1, "error", err)
	}
	return nil, fmt.Errorf("store: sin conexión tras %d intentos", retries)
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	m, err := connectWithRetry(ctx, logger, cfg.DBDSN, cfg.DBConnectRetries)
	if err != nil {
		return err
	}
	defer m.Close()
	logger.Info("conectado a MongoDB y migraciones aplicadas")

	feed, err := notify.New(cfg.FeedAddr, cfg.FeedTimeout)
	if err != nil {
		return fmt.Errorf("no se pudo crear cliente de Feed: %w", err)
	}
	defer feed.Close()

	healthSrv := health.NewServer()
	healthSrv.SetServingStatus("", healthpb.HealthCheckResponse_SERVING)

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

	svc := service.New(m, feed)
	srv := grpc.NewServer(server.UnaryInterceptors(cfg.RPCTimeout, logger))
	pb.RegisterPostServiceServer(srv, handler.NewPostServer(svc))
	healthpb.RegisterHealthServer(srv, healthSrv)
	reflection.Register(srv)

	lis, err := net.Listen("tcp", fmt.Sprintf(":%d", cfg.GRPCPort))
	if err != nil {
		return fmt.Errorf("no se pudo escuchar en :%d: %w", cfg.GRPCPort, err)
	}
	logger.Info("servidor gRPC escuchando", "puerto", cfg.GRPCPort)

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(lis) }()

	<-ctx.Done()
	logger.Info("apagando: drenando peticiones activas")

	stopped := make(chan struct{})
	go func() {
		srv.GracefulStop()
		close(stopped)
	}()
	select {
	case <-stopped:
	case <-time.After(cfg.ShutdownTimeout):
		logger.Warn("drenado excedió el tiempo límite, forzando parada")
		srv.Stop()
	}

	if err := <-serveErr; err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return err
	}
	return nil
}

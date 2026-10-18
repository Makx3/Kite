// Package server ensambla el servidor gRPC con interceptores de resiliencia.
package server

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func UnaryInterceptors(rpcTimeout time.Duration, logger *slog.Logger) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		timeoutInterceptor(rpcTimeout),
		recoveryInterceptor(logger),
		loggingInterceptor(logger),
	)
}

func timeoutInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(ctx, req)
	}
}

func recoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recuperado", "metodo", info.FullMethod, "panic", r, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "error interno del servicio")
			}
		}()
		return handler(ctx, req)
	}
}

func loggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logger.Info("rpc", "metodo", info.FullMethod, "duracion_ms", time.Since(start).Milliseconds(), "codigo", status.Code(err).String())
		return resp, err
	}
}

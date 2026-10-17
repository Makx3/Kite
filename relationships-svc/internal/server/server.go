// Package server ensambla el servidor gRPC con los interceptores de
// resiliencia: tiempo límite por RPC, recuperación de panics y
// logging estructurado. Un panic en cualquier handler se convierte
// en un error Internal; el servicio nunca se cae por una petición.
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

// UnaryInterceptors arma la cadena de interceptores unary.
func UnaryInterceptors(rpcTimeout time.Duration, logger *slog.Logger) grpc.ServerOption {
	return grpc.ChainUnaryInterceptor(
		timeoutInterceptor(rpcTimeout),
		recoveryInterceptor(logger),
		loggingInterceptor(logger),
	)
}

// timeoutInterceptor impone un tiempo máximo por invocación si el
// caller no puso uno más corto.
func timeoutInterceptor(d time.Duration) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		ctx, cancel := context.WithTimeout(ctx, d)
		defer cancel()
		return handler(ctx, req)
	}
}

// recoveryInterceptor convierte panics en codes.Internal.
func recoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (resp any, err error) {
		defer func() {
			if r := recover(); r != nil {
				logger.Error("panic recuperado",
					"metodo", info.FullMethod,
					"panic", r,
					"stack", string(debug.Stack()),
				)
				err = status.Error(codes.Internal, "error interno del servicio")
			}
		}()
		return handler(ctx, req)
	}
}

// loggingInterceptor registra método, duración y código de cada RPC.
func loggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req any,
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (any, error) {
		start := time.Now()
		resp, err := handler(ctx, req)
		logger.Info("rpc",
			"metodo", info.FullMethod,
			"duracion_ms", time.Since(start).Milliseconds(),
			"codigo", status.Code(err).String(),
		)
		return resp, err
	}
}

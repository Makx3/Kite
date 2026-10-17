// Package clients agrupa los adaptadores gRPC salientes del API Gateway.
// Cada servicio de negocio tiene su propio cliente: el gateway los consume
// por gRPC y nunca toca sus bases de datos.
//
// Las conexiones son perezosas (grpc.NewClient no conecta de inmediato),
// de modo que el gateway arranca aunque un servicio esté caído; el fallo
// aparece en la primera llamada y se traduce a un 503 claro.
package clients

import (
	"fmt"

	"google.golang.org/grpc"
	"google.golang.org/grpc/connectivity"
	"google.golang.org/grpc/credentials/insecure"
)

// dial abre una conexión gRPC hacia addr. El tráfico interno del sistema
// no sale de la red de compose, por eso no se negocia TLS aquí.
func dial(addr string) (*grpc.ClientConn, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("clients: no se pudo crear el cliente hacia %q: %w", addr, err)
	}
	return conn, nil
}

// stateOf reporta el estado de conectividad de la conexión para el
// health check. Es informativo: IDLE solo significa que aún no se usó.
func stateOf(conn *grpc.ClientConn) string {
	return connectivityStateName(conn.GetState())
}

func connectivityStateName(s connectivity.State) string {
	switch s {
	case connectivity.Idle:
		return "IDLE"
	case connectivity.Connecting:
		return "CONNECTING"
	case connectivity.Ready:
		return "READY"
	case connectivity.TransientFailure:
		return "TRANSIENT_FAILURE"
	case connectivity.Shutdown:
		return "SHUTDOWN"
	default:
		return "UNKNOWN"
	}
}

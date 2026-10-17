// Package notify implementa el puerto de notificaciones (service.Notifier)
// sobre un cliente gRPC del servicio de Notificaciones. Es un adaptador de
// salida: la capa de negocio no depende de gRPC, solo de la interfaz que
// declara y que este paquete satisface.
package notify

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	notificationspb "github.com/Makx3/Kite/relationships-svc/pb/notifications"
)

// Client publica alertas hacia el servicio de Notificaciones.
type Client struct {
	conn    *grpc.ClientConn
	client  notificationspb.NotificationServiceClient
	timeout time.Duration
}

// New crea el cliente. La conexión es perezosa: grpc.NewClient no conecta
// de inmediato, así que Relationships arranca aunque Notifications no esté
// disponible; el fallo aparece en la primera llamada y la capa de servicio
// lo degrada sin romper el seguimiento.
func New(addr string, timeout time.Duration) (*Client, error) {
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("notify: no se pudo crear el cliente hacia %q: %w", addr, err)
	}
	return &Client{
		conn:    conn,
		client:  notificationspb.NewNotificationServiceClient(conn),
		timeout: timeout,
	}, nil
}

// NotifyNewFollower avisa a Notifications que followerID empezó a seguir a
// followeeID. Usa su propio tiempo límite para no arrastrar la latencia del
// RPC entrante si Notifications responde lento.
func (c *Client) NotifyNewFollower(ctx context.Context, followerID, followeeID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.NotifyNewFollower(ctx, &notificationspb.NotifyNewFollowerRequest{
		FollowerId: followerID,
		FolloweeId: followeeID,
	})
	return err
}

// Close libera la conexión gRPC.
func (c *Client) Close() error { return c.conn.Close() }

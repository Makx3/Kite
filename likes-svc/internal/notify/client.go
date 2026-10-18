// Package notify implementa el puerto de notificaciones
// (service.Notifier) sobre un cliente gRPC del servicio Notifications.
// Es un adaptador de salida: la capa de negocio no depende de gRPC,
// solo de la interfaz que declara y que este paquete satisface.
package notify

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	notificationspb "github.com/Makx3/Kite/likes-svc/pb/notifications"
)

// Client publica alertas hacia el servicio de Notificaciones.
type Client struct {
	conn    *grpc.ClientConn
	client  notificationspb.NotificationServiceClient
	timeout time.Duration
}

// New crea el cliente. La conexión es perezosa: grpc.NewClient no
// conecta de inmediato, así que Likes arranca aunque Notifications no
// esté disponible. El MinConnectTimeout de 2s acorta los ~20s por
// defecto de gRPC para que un servicio caído se note rápido y la
// degradación ocurra antes de que expire el deadline del like.
func New(addr string, timeout time.Duration) (*Client, error) {
	params := grpc.ConnectParams{
		Backoff:           backoff.DefaultConfig,
		MinConnectTimeout: 2 * time.Second,
	}
	conn, err := grpc.NewClient(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithConnectParams(params),
	)
	if err != nil {
		return nil, fmt.Errorf("notify: no se pudo crear el cliente hacia %q: %w", addr, err)
	}
	return &Client{
		conn:    conn,
		client:  notificationspb.NewNotificationServiceClient(conn),
		timeout: timeout,
	}, nil
}

// NotifyPostLiked avisa a Notifications que userID reaccionó a un post
// de postAuthorID. Usa su propio tiempo límite para no arrastrar la
// latencia del RPC entrante si Notifications responde lento.
func (c *Client) NotifyPostLiked(ctx context.Context, postID, likerID, postAuthorID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.NotifyPostLiked(ctx, &notificationspb.NotifyPostLikedRequest{
		PostId:       postID,
		LikerId:      likerID,
		PostAuthorId: postAuthorID,
	})
	return err
}

// Close libera la conexión gRPC.
func (c *Client) Close() error { return c.conn.Close() }

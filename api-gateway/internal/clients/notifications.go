package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	notificationspb "github.com/Makx3/Kite/api-gateway/pb/notifications"
)

// Notifications es el cliente gRPC del servicio de alertas.
type Notifications struct {
	conn    *grpc.ClientConn
	client  notificationspb.NotificationServiceClient
	timeout time.Duration
}

// NewNotifications crea el cliente. No conecta de inmediato.
func NewNotifications(addr string, timeout time.Duration) (*Notifications, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Notifications{
		conn:    conn,
		client:  notificationspb.NewNotificationServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Unread lista las notificaciones sin leer de userID (paginado).
func (c *Notifications) Unread(ctx context.Context, userID string, limit, offset int32) ([]*notificationspb.Notification, int32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUnreadNotifications(ctx, &notificationspb.GetUnreadNotificationsRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return resp.GetNotifications(), resp.GetTotal(), nil
}

// MarkRead marca una notificación como leída.
func (c *Notifications) MarkRead(ctx context.Context, userID, notificationID string) (*notificationspb.Notification, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.MarkAsRead(ctx, &notificationspb.MarkAsReadRequest{
		UserId:         userID,
		NotificationId: notificationID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetNotification(), nil
}

// State reporta el estado de conectividad (para el health check).
func (c *Notifications) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Notifications) Close() error { return c.conn.Close() }

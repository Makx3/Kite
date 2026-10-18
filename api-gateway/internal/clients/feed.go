package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	feedpb "github.com/Makx3/Kite/api-gateway/pb/feed"
)

// Feed es el cliente gRPC del muro personal por usuario.
type Feed struct {
	conn    *grpc.ClientConn
	client  feedpb.FeedServiceClient
	timeout time.Duration
}

// NewFeed crea el cliente. No conecta de inmediato.
func NewFeed(addr string, timeout time.Duration) (*Feed, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Feed{
		conn:    conn,
		client:  feedpb.NewFeedServiceClient(conn),
		timeout: timeout,
	}, nil
}

// UserFeed devuelve la página de identificadores del muro de userID,
// de más reciente a más antiguo, con el total y la fecha del post más
// reciente. El contenido lo resuelve el gateway con Posts.
func (c *Feed) UserFeed(ctx context.Context, userID string, limit, offset int32) ([]string, int32, string, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetUserFeed(ctx, &feedpb.GetUserFeedRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, "", err
	}
	return resp.GetPostIds(), resp.GetTotal(), resp.GetLastUpdated(), nil
}

// State reporta el estado de conectividad (para el health check).
func (c *Feed) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Feed) Close() error { return c.conn.Close() }

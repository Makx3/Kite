// Package notify implementa el puerto de refresco de feed (service.FeedRefresher)
// sobre un cliente gRPC del servicio Feed. Es un adaptador de salida: la capa
// de negocio no depende de gRPC, solo de la interfaz que declara.
package notify

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/backoff"
	"google.golang.org/grpc/credentials/insecure"

	feedpb "github.com/Makx3/Kite/posts-svc/pb/feed"
)

// Client publica publicaciones nuevas hacia el servicio Feed.
type Client struct {
	conn    *grpc.ClientConn
	client  feedpb.FeedServiceClient
	timeout time.Duration
}

// New crea el cliente. La conexión es perezosa: grpc.NewClient no conecta
// de inmediato, así que Posts arranca aunque Feed no esté disponible. El
// MinConnectTimeout de 2s acorta los ~20s por defecto de gRPC para que el
// error llegue antes de que expire el deadline del post.
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
		client:  feedpb.NewFeedServiceClient(conn),
		timeout: timeout,
	}, nil
}

// RefreshFeed notifica a Feed que authorID creó el postID en createdAt.
func (c *Client) RefreshFeed(ctx context.Context, authorID, postID string, createdAt time.Time) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.RefreshFeed(ctx, &feedpb.RefreshFeedRequest{
		AuthorId:  authorID,
		PostId:    postID,
		CreatedAt: createdAt.Unix(),
	})
	return err
}

// Close libera la conexión gRPC.
func (c *Client) Close() error { return c.conn.Close() }

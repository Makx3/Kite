package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	postspb "github.com/Makx3/Kite/api-gateway/pb/posts"
)

// Posts es el cliente gRPC del servicio de publicaciones.
type Posts struct {
	conn    *grpc.ClientConn
	client  postspb.PostServiceClient
	timeout time.Duration
}

// NewPosts crea el cliente. No conecta de inmediato.
func NewPosts(addr string, timeout time.Duration) (*Posts, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Posts{
		conn:    conn,
		client:  postspb.NewPostServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Create publica content en nombre de userID.
func (c *Posts) Create(ctx context.Context, userID, content string) (*postspb.Post, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.CreatePost(ctx, &postspb.CreatePostRequest{
		UserId:  userID,
		Content: content,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetPost(), nil
}

// Get devuelve la publicación postID.
func (c *Posts) Get(ctx context.Context, postID string) (*postspb.Post, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetPost(ctx, &postspb.GetPostRequest{PostId: postID})
	if err != nil {
		return nil, err
	}
	return resp.GetPost(), nil
}

// GetByIDs hidrata un lote de publicaciones (agregación del feed). Los
// identificados inexistentes llegan en la lista `missing` del contrato.
func (c *Posts) GetByIDs(ctx context.Context, ids []string) ([]*postspb.Post, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetPostsByIds(ctx, &postspb.GetPostsByIdsRequest{PostIds: ids})
	if err != nil {
		return nil, err
	}
	return resp.GetPosts(), nil
}

// State reporta el estado de conectividad (para el health check).
func (c *Posts) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Posts) Close() error { return c.conn.Close() }

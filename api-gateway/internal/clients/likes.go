package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	likespb "github.com/Makx3/Kite/api-gateway/pb/likes"
)

// Likes es el cliente gRPC del servicio de reacciones.
type Likes struct {
	conn    *grpc.ClientConn
	client  likespb.LikeServiceClient
	timeout time.Duration
}

// NewLikes crea el cliente. No conecta de inmediato.
func NewLikes(addr string, timeout time.Duration) (*Likes, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Likes{
		conn:    conn,
		client:  likespb.NewLikeServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Like registra la reacción de userID a un post de postAuthorID. El
// autor lo resuelve el gateway con Posts antes de llamar, para que
// Likes no dependa de ese servicio.
func (c *Likes) Like(ctx context.Context, postID, userID, postAuthorID string) (*likespb.LikePostResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.LikePost(ctx, &likespb.LikePostRequest{
		PostId:       postID,
		UserId:       userID,
		PostAuthorId: postAuthorID,
	})
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// Unlike elimina la reacción de userID al post.
func (c *Likes) Unlike(ctx context.Context, postID, userID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.UnlikePost(ctx, &likespb.UnlikePostRequest{
		PostId: postID,
		UserId: userID,
	})
	return err
}

// State reporta el estado de conectividad (para el health check).
func (c *Likes) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Likes) Close() error { return c.conn.Close() }

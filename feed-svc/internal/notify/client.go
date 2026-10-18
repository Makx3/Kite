// Package notify implementa el puerto de resolución de seguidores
// (service.FollowerResolver) sobre un cliente gRPC de Relationships.
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

	relationshipspb "github.com/Makx3/Kite/feed-svc/pb/relationships"
)

// Client consulta el grafo de seguidores hacia Relationships.
type Client struct {
	conn    *grpc.ClientConn
	client  relationshipspb.RelationshipServiceClient
	timeout time.Duration
}

// New crea el cliente. La conexión es perezosa: grpc.NewClient no
// conecta de inmediato, así que Feed arranca aunque Relationships no
// esté disponible. El MinConnectTimeout de 2s acorta los ~20s por
// defecto de gRPC para que un servicio caído se note rápido y la
// degradación ocurra antes de que expire el deadline del post.
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
		client:  relationshipspb.NewRelationshipServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Followers devuelve una página de user_id que siguen a userID y el
// total de seguidores. Usa su propio tiempo límite para no arrastrar
// la latencia del RPC entrante si Relationships responde lento.
func (c *Client) Followers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	res, err := c.client.GetFollowers(ctx, &relationshipspb.GetFollowersRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	ids := res.GetUserIds()
	if ids == nil {
		ids = []string{}
	}
	return ids, res.GetTotal(), nil
}

// Close libera la conexión gRPC.
func (c *Client) Close() error { return c.conn.Close() }

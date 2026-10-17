package clients

import (
	"context"
	"time"

	"google.golang.org/grpc"

	relationshipspb "github.com/Makx3/Kite/api-gateway/pb/relationships"
)

// Relationships es el cliente gRPC del grafo de seguimiento.
type Relationships struct {
	conn    *grpc.ClientConn
	client  relationshipspb.RelationshipServiceClient
	timeout time.Duration
}

// NewRelationships crea el cliente. No conecta de inmediato.
func NewRelationships(addr string, timeout time.Duration) (*Relationships, error) {
	conn, err := dial(addr)
	if err != nil {
		return nil, err
	}
	return &Relationships{
		conn:    conn,
		client:  relationshipspb.NewRelationshipServiceClient(conn),
		timeout: timeout,
	}, nil
}

// Follow crea la arista followerID -> followeeID.
func (c *Relationships) Follow(ctx context.Context, followerID, followeeID string) (*relationshipspb.Follow, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.FollowUser(ctx, &relationshipspb.FollowUserRequest{
		FollowerId: followerID,
		FolloweeId: followeeID,
	})
	if err != nil {
		return nil, err
	}
	return resp.GetFollow(), nil
}

// Unfollow elimina la arista followerID -> followeeID.
func (c *Relationships) Unfollow(ctx context.Context, followerID, followeeID string) error {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	_, err := c.client.UnfollowUser(ctx, &relationshipspb.UnfollowUserRequest{
		FollowerId: followerID,
		FolloweeId: followeeID,
	})
	return err
}

// Followers lista los seguidores de userID (paginado).
func (c *Relationships) Followers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetFollowers(ctx, &relationshipspb.GetFollowersRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return resp.GetUserIds(), resp.GetTotal(), nil
}

// Following lista a quiénes sigue userID (paginado).
func (c *Relationships) Following(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	resp, err := c.client.GetFollowing(ctx, &relationshipspb.GetFollowingRequest{
		UserId: userID,
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, 0, err
	}
	return resp.GetUserIds(), resp.GetTotal(), nil
}

// State reporta el estado de conectividad (para el health check).
func (c *Relationships) State() string { return stateOf(c.conn) }

// Close libera la conexión gRPC.
func (c *Relationships) Close() error { return c.conn.Close() }

package httpapi

import (
	"context"

	notificationspb "github.com/Makx3/Kite/api-gateway/pb/notifications"
	relationshipspb "github.com/Makx3/Kite/api-gateway/pb/relationships"
	userspb "github.com/Makx3/Kite/api-gateway/pb/users"
)

// UsersBackend es el puerto que el gateway necesita del servicio Users.
// Los handlers dependen de esta interfaz y no de la implementación gRPC,
// de modo que las pruebas usan dobles en memoria sin red ni contenedores.
type UsersBackend interface {
	Create(ctx context.Context, username, email, displayName string) (*userspb.User, error)
	Get(ctx context.Context, id string) (*userspb.User, error)
	Update(ctx context.Context, id string, displayName, email *string) (*userspb.User, error)
	State() string
}

// RelationshipsBackend es el puerto del servicio Relationships.
type RelationshipsBackend interface {
	Follow(ctx context.Context, followerID, followeeID string) (*relationshipspb.Follow, error)
	Unfollow(ctx context.Context, followerID, followeeID string) error
	Followers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error)
	Following(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error)
	State() string
}

// NotificationsBackend es el puerto del servicio Notifications.
type NotificationsBackend interface {
	Unread(ctx context.Context, userID string, limit, offset int32) ([]*notificationspb.Notification, int32, error)
	MarkRead(ctx context.Context, userID, notificationID string) (*notificationspb.Notification, error)
	State() string
}

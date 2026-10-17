package httpapi

import (
	"time"

	"google.golang.org/protobuf/types/known/timestamppb"

	notificationspb "github.com/Makx3/Kite/api-gateway/pb/notifications"
	relationshipspb "github.com/Makx3/Kite/api-gateway/pb/relationships"
	userspb "github.com/Makx3/Kite/api-gateway/pb/users"
)

// userDTO es la representación REST de un usuario. Contiene todos los
// campos del contrato del diseño (user_id, username, email, display_name,
// created_at) y añade updated_at, que el servicio sí persiste.
type userDTO struct {
	UserID      string `json:"user_id"`
	Username    string `json:"username"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

func toUserDTO(u *userspb.User) *userDTO {
	if u == nil {
		return nil
	}
	return &userDTO{
		UserID:      u.GetUserId(),
		Username:    u.GetUsername(),
		Email:       u.GetEmail(),
		DisplayName: u.GetDisplayName(),
		CreatedAt:   formatTS(u.GetCreatedAt()),
		UpdatedAt:   formatTS(u.GetUpdatedAt()),
	}
}

// followDTO es la representación REST de una arista del grafo.
type followDTO struct {
	FollowerID string `json:"follower_id"`
	FolloweeID string `json:"followee_id"`
	CreatedAt  string `json:"created_at"`
}

func toFollowDTO(f *relationshipspb.Follow) *followDTO {
	if f == nil {
		return nil
	}
	return &followDTO{
		FollowerID: f.GetFollowerId(),
		FolloweeID: f.GetFolloweeId(),
		CreatedAt:  formatTS(f.GetCreatedAt()),
	}
}

// userIDsDTO es la página de identificadores devuelta por los listados del
// grafo de seguimiento.
type userIDsDTO struct {
	UserIDs []string `json:"user_ids"`
	Total   int32    `json:"total"`
}

// notificationDTO es la representación REST de una alerta. SourceUser es
// el resultado de la agregación: el identificador se hidrata con los datos
// del servicio Users para que un solo GET /notifications sea suficiente.
type notificationDTO struct {
	NotificationID string   `json:"notification_id"`
	Type           string   `json:"type"`
	UserID         string   `json:"user_id"`
	SourceUserID   string   `json:"source_user_id"`
	SourceUser     *userDTO `json:"source_user,omitempty"`
	PostID         *string  `json:"post_id,omitempty"`
	IsRead         bool     `json:"is_read"`
	CreatedAt      string   `json:"created_at"`
}

func toNotificationDTO(n *notificationspb.Notification) *notificationDTO {
	if n == nil {
		return nil
	}
	dto := &notificationDTO{
		NotificationID: n.GetNotificationId(),
		Type:           notificationTypeName(n.GetType()),
		UserID:         n.GetUserId(),
		SourceUserID:   n.GetSourceUserId(),
		IsRead:         n.GetIsRead(),
		CreatedAt:      formatTS(n.GetCreatedAt()),
	}
	// post_id solo tiene valor en las alertas de tipo LIKE.
	if pid := n.GetPostId(); pid != "" {
		dto.PostID = &pid
	}
	return dto
}

// notificationTypeName traduce el enum del contrato al nombre sin prefijo,
// que es el mismo que persiste el servicio de Notificaciones.
func notificationTypeName(t notificationspb.NotificationType) string {
	switch t {
	case notificationspb.NotificationType_NOTIFICATION_TYPE_NEW_FOLLOWER:
		return "NEW_FOLLOWER"
	case notificationspb.NotificationType_NOTIFICATION_TYPE_LIKE:
		return "LIKE"
	default:
		return "UNSPECIFIED"
	}
}

// formatTS convierte un timestamp protobuf a RFC3339 en UTC; devuelve la
// cadena vacía si el campo no vino.
func formatTS(ts *timestamppb.Timestamp) string {
	if ts == nil || !ts.IsValid() {
		return ""
	}
	return ts.AsTime().UTC().Format(time.RFC3339Nano)
}

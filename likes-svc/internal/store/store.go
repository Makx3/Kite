// Package store define la interfaz del almacén de reacciones y la
// entidad interna que devuelve. No depende de gRPC ni del dominio de
// negocio: la capa de servicio traduce estos registros a entidades.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound se devuelve cuando la reacción no existe (p. ej. al
// quitar un like que no estaba puesto).
var ErrNotFound = errors.New("like no encontrado")

// LikeRecord es un documento de la colección likes tal como se
// persiste en MongoDB.
type LikeRecord struct {
	ID        string
	PostID    string
	UserID    string
	CreatedAt time.Time
}

// LikeStore es la interfaz de persistencia de las reacciones.
type LikeStore interface {
	// Create registra que userID reaccionó a postID. Es idempotente:
	// si la reacción ya existe devuelve la existente y created=false,
	// para que el servicio no vuelva a notificar el mismo like.
	Create(ctx context.Context, postID, userID string) (LikeRecord, bool, error)
	// Delete elimina la reacción; ErrNotFound si no existía.
	Delete(ctx context.Context, postID, userID string) error
	// ListByPost retorna una página de user_id que reaccionaron a
	// postID (de más antigua a más reciente) y el total de likes.
	ListByPost(ctx context.Context, postID string, limit, offset int32) ([]LikeRecord, int32, error)
	// Exists indica si userID ya reaccionó a postID.
	Exists(ctx context.Context, postID, userID string) (bool, error)
	// Ping verifica la conectividad con la base de datos.
	Ping(ctx context.Context) error
	// Close libera los recursos de la conexión.
	Close() error
}

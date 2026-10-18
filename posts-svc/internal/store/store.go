// Package store define la interfaz del almacén y las entidades
// internas compartidas. No depende de gRPC ni del dominio de negocio.
package store

import (
	"context"
	"errors"
	"time"
)

// Errores sentinela que mapea el servicio y el handler a códigos gRPC.
var (
	ErrNotFound = errors.New("post no encontrado")
	ErrInvalid  = errors.New("datos inválidos")
)

// PostRecord representa una fila de posts.
type PostRecord struct {
	ID        string
	UserID    string
	Content   string
	CreatedAt time.Time
}

// PostStore define las operaciones sobre posts_db.
type PostStore interface {
	Create(ctx context.Context, userID, content string) (PostRecord, error)
	Get(ctx context.Context, id string) (PostRecord, error)
	GetByIDs(ctx context.Context, ids []string) ([]PostRecord, error)
	GetByUser(ctx context.Context, userID string, limit, offset int32) ([]PostRecord, int32, error)
	Delete(ctx context.Context, id, userID string) error
	Ping(ctx context.Context) error
	Close() error
}

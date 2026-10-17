// Package store define la abstracción de persistencia del grafo de
// seguimiento y su implementación en PostgreSQL. La capa de servicio
// depende solo de la interfaz, de modo que los tests usan un doble en
// memoria y la implementación real puede cambiarse sin tocar el negocio.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound se devuelve cuando la relación no existe (p. ej. al
// dejar de seguir a alguien que no se seguía).
var ErrNotFound = errors.New("relación no encontrada")

// ErrInvalidID se devuelve cuando un identificador no tiene formato
// de UUID válido; el handler lo traduce a gRPC InvalidArgument.
var ErrInvalidID = errors.New("identificador con formato inválido")

// RelationshipStore es la interfaz de persistencia del grafo.
type RelationshipStore interface {
	// Create registra que followerID sigue a followeeID. Es
	// idempotente: si la relación ya existe, devuelve la existente sin
	// error. created indica si la arista es nueva, para que el servicio
	// no notifique dos veces el mismo seguimiento.
	Create(ctx context.Context, followerID, followeeID string) (FollowRecord, bool, error)
	// Delete elimina la relación; ErrNotFound si no existía.
	Delete(ctx context.Context, followerID, followeeID string) error
	// ListFollowers retorna una página de user_id que siguen a userID
	// (según limit/offset) más el total de seguidores.
	ListFollowers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error)
	// ListFollowing retorna una página de user_id a los que sigue userID
	// más el total.
	ListFollowing(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error)
	// Ping verifica la conectividad con la base de datos.
	Ping(ctx context.Context) error
	// Close libera los recursos del pool.
	Close()
}

// FollowRecord es la fila de la tabla follows tal como se persiste.
type FollowRecord struct {
	FollowerID string
	FolloweeID string
	CreatedAt  time.Time
}

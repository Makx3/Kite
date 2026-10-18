// Package store define la interfaz del almacén del muro y la entidad
// interna que devuelve. No depende de gRPC ni del dominio de negocio:
// la capa de servicio traduce estos registros a entidades de dominio.
package store

import (
	"context"
	"time"
)

// EntryRecord es una fila del muro tal como se persiste en Redis.
type EntryRecord struct {
	PostID    string
	CreatedAt time.Time
}

// FeedStore es la interfaz de persistencia del muro por usuario.
type FeedStore interface {
	// Add inserta postID en el muro de cada usuario de userIDs con
	// fecha createdAt. Si ttl es mayor que cero, cada muro expira a
	// los ttl de su última escritura.
	Add(ctx context.Context, userIDs []string, postID string, createdAt time.Time, ttl time.Duration) error
	// Page devuelve una página del muro de userID (de más reciente a
	// más antiguo) y el total de entradas del muro.
	Page(ctx context.Context, userID string, limit, offset int32) ([]EntryRecord, int32, error)
	// Ping verifica la conectividad con Redis.
	Ping(ctx context.Context) error
	// Close libera la conexión.
	Close() error
}

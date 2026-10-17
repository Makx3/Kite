// Package store define la abstracción de persistencia y su
// implementación en PostgreSQL. La capa de servicio depende solo
// de la interfaz, de modo que los tests usan un doble en memoria
// y la implementación real puede cambiarse sin tocar el negocio.
package store

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound se devuelve cuando el user_id no existe.
var ErrNotFound = errors.New("usuario no encontrado")

// ErrEmailTaken se devuelve cuando el email ya está registrado
// por otro usuario (restricción UNIQUE de la base de datos).
var ErrEmailTaken = errors.New("email ya registrado")

// UserStore es la interfaz de persistencia de usuarios.
type UserStore interface {
	// Create inserta un usuario y devuelve la fila persistida,
	// incluyendo el ID y timestamps generados por la base de datos.
	Create(ctx context.Context, username, email string) (UserRecord, error)
	// Get retorna el usuario o ErrNotFound.
	Get(ctx context.Context, id string) (UserRecord, error)
	// Update aplica un parche: solo los punteros no-nulos se
	// modifican. Retorna ErrNotFound si el id no existe.
	Update(ctx context.Context, id string, username, email *string) (UserRecord, error)
	// Ping verifica la conectividad con la base de datos.
	Ping(ctx context.Context) error
	// Close libera los recursos del pool.
	Close()
}

// UserRecord es la fila de usuarios tal como se persiste.
type UserRecord struct {
	ID        string
	Username  string
	Email     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Package domain contiene las entidades de negocio del servicio.
// No depende de gRPC, ni de MongoDB, ni del transporte.
package domain

import "time"

// Like es la reacción de un usuario a una publicación. Los
// identificadores son referencias lógicas a los servicios Posts y
// Users; este servicio no los valida ni mantiene claves foráneas
// hacia otra base de datos.
type Like struct {
	ID        string
	PostID    string
	UserID    string
	CreatedAt time.Time
}

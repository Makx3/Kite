// Package domain contiene las entidades de negocio del servicio Posts.
// No depende de gRPC, ni de MongoDB, ni del transporte: es el núcleo
// estable de la lógica de negocio.
package domain

import "time"

// Post es la publicación creada por un usuario. El ID es el ObjectID
// de MongoDB serializado a string en la capa de transporte/proto, para
// mantener el dominio independiente del driver.
type Post struct {
	ID        string
	UserID    string
	Content   string
	CreatedAt time.Time
}

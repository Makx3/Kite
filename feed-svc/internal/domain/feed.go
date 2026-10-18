// Package domain contiene las entidades de negocio del servicio.
// No depende de gRPC, ni de Redis, ni del transporte.
package domain

import "time"

// Entry es una publicación dentro del muro de un usuario. Feed guarda
// solo el identificador y su instante de creación: el contenido vive
// en Posts y lo resuelve el API Gateway al aggregar la respuesta.
type Entry struct {
	UserID    string
	PostID    string
	CreatedAt time.Time
}

// Page es una página del muro de un usuario, de más reciente a más
// antiguo, con el total real de entradas y la fecha del post más
// reciente (vacía si el muro no tiene nada).
type Page struct {
	Entries     []Entry
	Total       int32
	LastUpdated string
}

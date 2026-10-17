// Package domain contiene las entidades de negocio del servicio.
// No depende de gRPC, ni de la base de datos, ni del transporte:
// es el núcleo estable alrededor del cual se construyen las capas.
package domain

import "time"

// User es la entidad de identidad del sistema. ID es el UUID
// generado por la base de datos (texto para no acoplar el dominio
// al driver de PostgreSQL). Username es el identificador estable y
// no se edita; DisplayName es el nombre visible del usuario.
type User struct {
	ID          string
	Username    string
	Email       string
	DisplayName string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Package domain contiene las entidades de negocio del servicio.
// No depende de gRPC, ni de la base de datos, ni del transporte.
package domain

import "time"

// Follow es una arista del grafo de seguimiento: FollowerID sigue a
// FolloweeID desde CreatedAt. Los identificadores son referencias
// lógicas al servicio de Usuarios; este servicio no los valida ni
// mantiene claves foráneas hacia otra base de datos.
type Follow struct {
	FollowerID string
	FolloweeID string
	CreatedAt  time.Time
}

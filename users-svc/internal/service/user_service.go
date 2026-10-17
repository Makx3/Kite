// Package service contiene la lógica de negocio del dominio User.
// Valida entradas, aplica reglas y orquesta el store; no conoce
// gRPC ni PostgreSQL.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"unicode/utf8"

	"github.com/Makx3/Kite/users-svc/internal/domain"
	"github.com/Makx3/Kite/users-svc/internal/store"
)

// ErrValidation marca errores de entrada del caller; el handler los
// traduce a gRPC InvalidArgument.
var ErrValidation = errors.New("datos inválidos")

// Límites de longitud alineados con el diseño (username VARCHAR(50),
// display_name VARCHAR(80)). El mínimo de username es una regla propia
// razonable: el diseño solo fija el máximo.
const (
	usernameMin    = 3
	usernameMax    = 50
	displayNameMin = 1
	displayNameMax = 80
)

// UserService orquesta las operaciones sobre la entidad User.
type UserService struct {
	store store.UserStore
}

// New inyecta el store requerido.
func New(s store.UserStore) *UserService { return &UserService{store: s} }

// CreateUser valida la entrada y persiste el usuario.
func (s *UserService) CreateUser(ctx context.Context, username, email, displayName string) (domain.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	displayName = strings.TrimSpace(displayName)
	if err := validateUser(username, email, displayName); err != nil {
		return domain.User{}, err
	}

	rec, err := s.store.Create(ctx, username, email, displayName)
	if err != nil {
		return domain.User{}, err
	}
	return toDomain(rec), nil
}

// GetUser retorna el usuario o store.ErrNotFound.
func (s *UserService) GetUser(ctx context.Context, id string) (domain.User, error) {
	if strings.TrimSpace(id) == "" {
		return domain.User{}, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	rec, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.User{}, err
	}
	return toDomain(rec), nil
}

// UpdateUser aplica un parche con los campos no-nulos (display_name
// y/o email). Exige al menos un campo y valida cada uno presente. El
// username no se edita: es el identificador estable del usuario.
func (s *UserService) UpdateUser(ctx context.Context, id string, displayName, email *string) (domain.User, error) {
	if strings.TrimSpace(id) == "" {
		return domain.User{}, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	if displayName == nil && email == nil {
		return domain.User{}, fmt.Errorf("%w: se debe indicar display_name o email", ErrValidation)
	}

	// Normaliza y valida solo lo presente.
	finalName, finalEmail := displayName, email
	if displayName != nil {
		v := strings.TrimSpace(*displayName)
		if n := utf8.RuneCountInString(v); n < displayNameMin || n > displayNameMax {
			return domain.User{}, fmt.Errorf("%w: display_name debe tener entre %d y %d caracteres", ErrValidation, displayNameMin, displayNameMax)
		}
		finalName = &v
	}
	if email != nil {
		v := strings.TrimSpace(*email)
		if _, err := mail.ParseAddress(v); err != nil {
			return domain.User{}, fmt.Errorf("%w: email con formato inválido", ErrValidation)
		}
		finalEmail = &v
	}

	rec, err := s.store.Update(ctx, id, finalName, finalEmail)
	if err != nil {
		return domain.User{}, err
	}
	return toDomain(rec), nil
}

// validateUser aplica las reglas de creación.
func validateUser(username, email, displayName string) error {
	if n := utf8.RuneCountInString(username); n < usernameMin || n > usernameMax {
		return fmt.Errorf("%w: username debe tener entre %d y %d caracteres", ErrValidation, usernameMin, usernameMax)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: email con formato inválido", ErrValidation)
	}
	if n := utf8.RuneCountInString(displayName); n < displayNameMin || n > displayNameMax {
		return fmt.Errorf("%w: display_name debe tener entre %d y %d caracteres", ErrValidation, displayNameMin, displayNameMax)
	}
	return nil
}

// toDomain convierte la fila del store en la entidad de negocio.
func toDomain(rec store.UserRecord) domain.User {
	return domain.User{
		ID:          rec.ID,
		Username:    rec.Username,
		Email:       rec.Email,
		DisplayName: rec.DisplayName,
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
}

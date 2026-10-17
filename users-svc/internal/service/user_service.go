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

// usernameMin/Max acotan la longitud del nombre de usuario.
const (
	usernameMin = 3
	usernameMax = 32
)

// UserService orquesta las operaciones sobre la entidad User.
type UserService struct {
	store store.UserStore
}

// New inyecta el store requerido.
func New(s store.UserStore) *UserService { return &UserService{store: s} }

// CreateUser valida la entrada y persiste el usuario.
func (s *UserService) CreateUser(ctx context.Context, username, email string) (domain.User, error) {
	username = strings.TrimSpace(username)
	email = strings.TrimSpace(email)
	if err := validateUser(username, email); err != nil {
		return domain.User{}, err
	}

	rec, err := s.store.Create(ctx, username, email)
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

// UpdateUser aplica un parche con los campos no-nulos. exige al
// menos un campo y valida cada uno presente.
func (s *UserService) UpdateUser(ctx context.Context, id string, username, email *string) (domain.User, error) {
	if strings.TrimSpace(id) == "" {
		return domain.User{}, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	if username == nil && email == nil {
		return domain.User{}, fmt.Errorf("%w: se debe indicar username o email", ErrValidation)
	}

	// Normaliza y valida solo lo presente.
	finalUser, finalEmail := username, email
	if username != nil {
		v := strings.TrimSpace(*username)
		if n := utf8.RuneCountInString(v); n < usernameMin || n > usernameMax {
			return domain.User{}, fmt.Errorf("%w: username debe tener entre %d y %d caracteres", ErrValidation, usernameMin, usernameMax)
		}
		finalUser = &v
	}
	if email != nil {
		v := strings.TrimSpace(*email)
		if _, err := mail.ParseAddress(v); err != nil {
			return domain.User{}, fmt.Errorf("%w: email con formato inválido", ErrValidation)
		}
		finalEmail = &v
	}

	rec, err := s.store.Update(ctx, id, finalUser, finalEmail)
	if err != nil {
		return domain.User{}, err
	}
	return toDomain(rec), nil
}

// validateUser aplica las reglas de creación.
func validateUser(username, email string) error {
	if n := utf8.RuneCountInString(username); n < usernameMin || n > usernameMax {
		return fmt.Errorf("%w: username debe tener entre %d y %d caracteres", ErrValidation, usernameMin, usernameMax)
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return fmt.Errorf("%w: email con formato inválido", ErrValidation)
	}
	return nil
}

// toDomain convierte la fila del store en la entidad de negocio.
func toDomain(rec store.UserRecord) domain.User {
	return domain.User{
		ID:        rec.ID,
		Username:  rec.Username,
		Email:     rec.Email,
		CreatedAt: rec.CreatedAt,
		UpdatedAt: rec.UpdatedAt,
	}
}

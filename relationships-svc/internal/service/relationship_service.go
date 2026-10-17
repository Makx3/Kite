// Package service contiene la lógica de negocio del grafo de
// seguimiento. Valida entradas, aplica reglas y orquesta el store y la
// publicación de alertas; no conoce gRPC ni PostgreSQL.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Makx3/Kite/relationships-svc/internal/domain"
	"github.com/Makx3/Kite/relationships-svc/internal/store"
)

// ErrValidation marca errores de entrada del caller; el handler los
// traduce a gRPC InvalidArgument.
var ErrValidation = errors.New("datos inválidos")

// Política de paginación: limit=0 significa "usa el valor por defecto" y
// cualquier limit mayor que el máximo se recorta a él, para proteger la
// base de datos de consultas desmedidas.
const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// Notifier publica alertas en el servicio de Notificaciones. La interfaz
// se declara aquí (lado consumidor) para poder sustituirla por un doble en
// las pruebas y para no acoplar el negocio al cliente gRPC concreto.
type Notifier interface {
	NotifyNewFollower(ctx context.Context, followerID, followeeID string) error
}

// RelationshipService orquesta las operaciones sobre el grafo.
type RelationshipService struct {
	store    store.RelationshipStore
	notifier Notifier
	logger   *slog.Logger
}

// New inyecta el store, el publicador de alertas y el logger. Si notifier
// es nil el disparador queda deshabilitado; si logger es nil se usa el
// logger por defecto de slog.
func New(s store.RelationshipStore, notifier Notifier, logger *slog.Logger) *RelationshipService {
	if logger == nil {
		logger = slog.Default()
	}
	return &RelationshipService{store: s, notifier: notifier, logger: logger}
}

// FollowUser registra el seguimiento. Es idempotente: seguir a quien ya se
// sigue devuelve la relación existente sin error. Cuando la arista es
// nueva dispara Notifications.NotifyNewFollower; si esa llamada falla, el
// seguimiento ya quedó persistido y el error no se propaga (degradación
// elegante: que Notifications esté caído no rompe el grafo).
func (s *RelationshipService) FollowUser(ctx context.Context, followerID, followeeID string) (domain.Follow, error) {
	followerID = strings.TrimSpace(followerID)
	followeeID = strings.TrimSpace(followeeID)
	if err := validatePair(followerID, followeeID); err != nil {
		return domain.Follow{}, err
	}

	rec, created, err := s.store.Create(ctx, followerID, followeeID)
	if err != nil {
		return domain.Follow{}, err
	}

	if created && s.notifier != nil {
		if err := s.notifier.NotifyNewFollower(ctx, followerID, followeeID); err != nil {
			s.logger.Warn("no se pudo notificar el nuevo seguidor",
				"follower_id", followerID, "followee_id", followeeID, "error", err)
		}
	}
	return toDomain(rec), nil
}

// UnfollowUser elimina el seguimiento; store.ErrNotFound si no existía.
func (s *RelationshipService) UnfollowUser(ctx context.Context, followerID, followeeID string) error {
	followerID = strings.TrimSpace(followerID)
	followeeID = strings.TrimSpace(followeeID)
	if err := validatePair(followerID, followeeID); err != nil {
		return err
	}
	return s.store.Delete(ctx, followerID, followeeID)
}

// GetFollowers retorna una página de seguidores de userID más el total.
func (s *RelationshipService) GetFollowers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	if err := validateID(userID); err != nil {
		return nil, 0, err
	}
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListFollowers(ctx, strings.TrimSpace(userID), limit, offset)
}

// GetFollowing retorna una página de a quiénes sigue userID más el total.
func (s *RelationshipService) GetFollowing(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	if err := validateID(userID); err != nil {
		return nil, 0, err
	}
	limit, offset, err := normalizePage(limit, offset)
	if err != nil {
		return nil, 0, err
	}
	return s.store.ListFollowing(ctx, strings.TrimSpace(userID), limit, offset)
}

// normalizePage aplica la política de paginación: rechaza negativos, usa el
// valor por defecto cuando limit=0 y recorta los límites excesivos.
func normalizePage(limit, offset int32) (int32, int32, error) {
	if limit < 0 {
		return 0, 0, fmt.Errorf("%w: limit no puede ser negativo", ErrValidation)
	}
	if offset < 0 {
		return 0, 0, fmt.Errorf("%w: offset no puede ser negativo", ErrValidation)
	}
	if limit == 0 {
		limit = defaultPageLimit
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return limit, offset, nil
}

// validatePair valida un par (follower, followee): ambos obligatorios y
// distintos (nadie puede seguirse a sí mismo).
func validatePair(followerID, followeeID string) error {
	if err := validateID(followerID); err != nil {
		return fmt.Errorf("%w: follower_id es obligatorio", ErrValidation)
	}
	if err := validateID(followeeID); err != nil {
		return fmt.Errorf("%w: followee_id es obligatorio", ErrValidation)
	}
	if followerID == followeeID {
		return fmt.Errorf("%w: un usuario no puede seguirse a sí mismo", ErrValidation)
	}
	return nil
}

// validateID exige un identificador no vacío.
func validateID(id string) error {
	if strings.TrimSpace(id) == "" {
		return ErrValidation
	}
	return nil
}

// toDomain convierte la fila del store en la entidad de negocio.
func toDomain(rec store.FollowRecord) domain.Follow {
	return domain.Follow{
		FollowerID: rec.FollowerID,
		FolloweeID: rec.FolloweeID,
		CreatedAt:  rec.CreatedAt,
	}
}

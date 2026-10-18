// Package service contiene la lógica de negocio de las reacciones.
// Valida entradas, aplica reglas y orquesta el store; no conoce gRPC
// ni MongoDB.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/Makx3/Kite/likes-svc/internal/domain"
	"github.com/Makx3/Kite/likes-svc/internal/store"
)

var ErrValidation = errors.New("datos inválidos")

const (
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// Notifier publica la alerta de like hacia Notifications.
type Notifier interface {
	NotifyPostLiked(ctx context.Context, postID, likerID, postAuthorID string) error
}

// LikeService orquesta las operaciones sobre reacciones.
type LikeService struct {
	store    store.LikeStore
	notifier Notifier
	logger   *slog.Logger
}

// New inyecta el store, el publicador de alertas y el logger. Si
// notifier es nil el disparador queda deshabilitado; si logger es nil
// se usa el logger por defecto de slog.
func New(s store.LikeStore, notifier Notifier, logger *slog.Logger) *LikeService {
	if logger == nil {
		logger = slog.Default()
	}
	return &LikeService{store: s, notifier: notifier, logger: logger}
}

// LikePost registra la reacción de userID a un post de postAuthorID.
// Es idempotente: si el usuario ya reaccionó se devuelve el like
// existente y no se vuelve a notificar (mismo criterio que FollowUser
// en Relationships). Cuando la reacción es nueva invoca
// Notifications.NotifyPostLiked; si esa llamada falla, el like ya
// quedó persistido y el error no se propaga (degradación elegante:
// que Notifications esté caído no impide reaccionar). Notifications
// es quien decide no crear la alerta cuando el autor reacciona a su
// propia publicación. Devuelve el like y si fue nuevo.
func (s *LikeService) LikePost(ctx context.Context, postID, userID, postAuthorID string) (domain.Like, bool, error) {
	postID = strings.TrimSpace(postID)
	userID = strings.TrimSpace(userID)
	postAuthorID = strings.TrimSpace(postAuthorID)
	if postID == "" {
		return domain.Like{}, false, fmt.Errorf("%w: post_id es obligatorio", ErrValidation)
	}
	if userID == "" {
		return domain.Like{}, false, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	if postAuthorID == "" {
		return domain.Like{}, false, fmt.Errorf("%w: post_author_id es obligatorio", ErrValidation)
	}

	rec, created, err := s.store.Create(ctx, postID, userID)
	if err != nil {
		return domain.Like{}, false, err
	}

	if created && s.notifier != nil {
		if err := s.notifier.NotifyPostLiked(ctx, postID, userID, postAuthorID); err != nil {
			s.logger.Warn("no se pudo notificar el like",
				"post_id", postID, "liker_id", userID, "error", err)
		}
	}
	return toDomain(rec), created, nil
}

// UnlikePost elimina la reacción; store.ErrNotFound si no existía.
func (s *LikeService) UnlikePost(ctx context.Context, postID, userID string) error {
	postID = strings.TrimSpace(postID)
	userID = strings.TrimSpace(userID)
	if postID == "" || userID == "" {
		return fmt.Errorf("%w: post_id y user_id son obligatorios", ErrValidation)
	}
	return s.store.Delete(ctx, postID, userID)
}

// GetPostLikes devuelve una página de user_id que reaccionaron a
// postID y el total de reacciones. Un post sin likes no es un error:
// devuelve la página vacía con total 0.
func (s *LikeService) GetPostLikes(ctx context.Context, postID string, limit, offset int32) ([]string, int32, error) {
	postID = strings.TrimSpace(postID)
	if postID == "" {
		return nil, 0, fmt.Errorf("%w: post_id es obligatorio", ErrValidation)
	}
	l, o := normalizePage(limit, offset)
	recs, total, err := s.store.ListByPost(ctx, postID, l, o)
	if err != nil {
		return nil, 0, err
	}
	ids := make([]string, 0, len(recs))
	for _, r := range recs {
		ids = append(ids, r.UserID)
	}
	return ids, total, nil
}

// HasLiked indica si userID ya reaccionó a postID.
func (s *LikeService) HasLiked(ctx context.Context, postID, userID string) (bool, error) {
	postID = strings.TrimSpace(postID)
	userID = strings.TrimSpace(userID)
	if postID == "" || userID == "" {
		return false, fmt.Errorf("%w: post_id y user_id son obligatorios", ErrValidation)
	}
	return s.store.Exists(ctx, postID, userID)
}

func normalizePage(limit, offset int32) (int32, int32) {
	l := limit
	if l <= 0 || l > maxPageLimit {
		l = defaultPageLimit
	}
	o := offset
	if o < 0 {
		o = 0
	}
	return l, o
}

func toDomain(rec store.LikeRecord) domain.Like {
	return domain.Like{
		ID:        rec.ID,
		PostID:    rec.PostID,
		UserID:    rec.UserID,
		CreatedAt: rec.CreatedAt,
	}
}

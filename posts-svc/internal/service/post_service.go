// Package service contiene la lógica de negocio del dominio Post.
// Valida entradas, aplica reglas y orquesta el store; no conoce gRPC ni MongoDB.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Makx3/Kite/posts-svc/internal/domain"
	"github.com/Makx3/Kite/posts-svc/internal/store"
)

var ErrValidation = errors.New("datos inválidos")

const (
	contentMin       = 1
	contentMax       = 280
	defaultPageLimit = 50
	maxPageLimit     = 100
)

// FeedRefresher notifica a Feed que se creó un post.
type FeedRefresher interface {
	RefreshFeed(ctx context.Context, authorID, postID string, createdAt time.Time) error
}

// PostService orquesta las operaciones sobre publicaciones.
type PostService struct {
	store  store.PostStore
	feed   FeedRefresher
	logger *slog.Logger
}

// New inyecta el store, el refresco de feed y el logger. Si feed es nil
// el disparador queda deshabilitado; si logger es nil se usa el logger
// por defecto de slog.
func New(s store.PostStore, f FeedRefresher, logger *slog.Logger) *PostService {
	if logger == nil {
		logger = slog.Default()
	}
	return &PostService{store: s, feed: f, logger: logger}
}

// CreatePost persiste la publicación y luego la reparte en los feeds de
// sus seguidores (y el propio autor) mediante Feed.RefreshFeed. Si esa
// llamada falla, el post ya quedó guardado y el error no se propaga
// (degradación elegante: que Feed esté caído no impide publicar).
func (s *PostService) CreatePost(ctx context.Context, userID, content string) (domain.Post, error) {
	userID = strings.TrimSpace(userID)
	content = strings.TrimSpace(content)
	if userID == "" {
		return domain.Post{}, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	if n := utf8.RuneCountInString(content); n < contentMin || n > contentMax {
		return domain.Post{}, fmt.Errorf("%w: content debe tener entre %d y %d caracteres", ErrValidation, contentMin, contentMax)
	}

	rec, err := s.store.Create(ctx, userID, content)
	if err != nil {
		return domain.Post{}, err
	}
	post := toDomain(rec)

	if s.feed != nil {
		if err := s.feed.RefreshFeed(ctx, post.UserID, post.ID, post.CreatedAt); err != nil {
			s.logger.Warn("no se pudo refrescar el feed",
				"post_id", post.ID, "user_id", post.UserID, "error", err)
		}
	}
	return post, nil
}

func (s *PostService) GetPost(ctx context.Context, id string) (domain.Post, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Post{}, fmt.Errorf("%w: post_id es obligatorio", ErrValidation)
	}
	rec, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Post{}, err
	}
	return toDomain(rec), nil
}

func (s *PostService) GetPostsByIDs(ctx context.Context, ids []string) ([]domain.Post, []string, error) {
	res := []domain.Post{}
	found := map[string]bool{}
	recs, err := s.store.GetByIDs(ctx, ids)
	if err != nil {
		return nil, nil, err
	}
	for _, r := range recs {
		res = append(res, toDomain(r))
		found[r.ID] = true
	}
	missing := []string{}
	for _, id := range ids {
		if !found[id] {
			missing = append(missing, id)
		}
	}
	return res, missing, nil
}

func (s *PostService) GetPostsByUser(ctx context.Context, userID string, limit, offset int32) ([]domain.Post, int32, error) {
	if strings.TrimSpace(userID) == "" {
		return nil, 0, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	l, o := normalizePage(limit, offset)
	recs, total, err := s.store.GetByUser(ctx, userID, l, o)
	if err != nil {
		return nil, 0, err
	}
	res := make([]domain.Post, 0, len(recs))
	for _, r := range recs {
		res = append(res, toDomain(r))
	}
	return res, total, nil
}

func (s *PostService) DeletePost(ctx context.Context, postID, userID string) error {
	if strings.TrimSpace(postID) == "" || strings.TrimSpace(userID) == "" {
		return fmt.Errorf("%w: post_id y user_id son obligatorios", ErrValidation)
	}
	if err := s.store.Delete(ctx, postID, userID); err != nil {
		return err
	}
	return nil
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

func toDomain(rec store.PostRecord) domain.Post {
	return domain.Post{
		ID:        rec.ID,
		UserID:    rec.UserID,
		Content:   rec.Content,
		CreatedAt: rec.CreatedAt,
	}
}

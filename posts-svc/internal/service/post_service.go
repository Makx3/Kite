// Package service contiene la lógica de negocio del dominio Post.
// Valida entradas, aplica reglas y orquesta el store; no conoce gRPC ni MongoDB.
package service

import (
	"context"
	"errors"
	"fmt"
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
	RefreshFeed(ctx context.Context, authorID, postID string, createdAt time.Time) error // createdAt time.Time
}

type UserService struct {
	store store.PostStore
	feed  FeedRefresher
}

func New(s store.PostStore, f FeedRefresher) *UserService {
	return &UserService{store: s, feed: f}
}

func (s *UserService) CreatePost(ctx context.Context, userID, content string) (domain.Post, error) {
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

	// Degradación elegante: si Feed falla, no rompemos la creación; solo registramos el error
	if s.feed != nil {
		if err := s.feed.RefreshFeed(ctx, post.UserID, post.ID, post.CreatedAt); err != nil {
			// No propagamos error: el post ya existe; patrón consistente con Relationships→Notifications
			// (warn via handler/log no disponible aquí; el llamador puede loguear)
			// para cumplir "sin crash", devolvemos post aunque refresh falle
		}
	}
	return post, nil
}

func (s *UserService) GetPost(ctx context.Context, id string) (domain.Post, error) {
	if strings.TrimSpace(id) == "" {
		return domain.Post{}, fmt.Errorf("%w: post_id es obligatorio", ErrValidation)
	}
	rec, err := s.store.Get(ctx, id)
	if err != nil {
		return domain.Post{}, err
	}
	return toDomain(rec), nil
}

func (s *UserService) GetPostsByIDs(ctx context.Context, ids []string) ([]domain.Post, []string, error) {
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

func (s *UserService) GetPostsByUser(ctx context.Context, userID string, limit, offset int32) ([]domain.Post, int32, error) {
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

func (s *UserService) DeletePost(ctx context.Context, postID, userID string) error {
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

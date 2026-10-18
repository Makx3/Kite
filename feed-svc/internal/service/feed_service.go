// Package service contiene la lógica de negocio del muro por usuario.
// Valida entradas, aplica reglas y orquesta el store; no conoce gRPC
// ni Redis.
package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/Makx3/Kite/feed-svc/internal/domain"
	"github.com/Makx3/Kite/feed-svc/internal/store"
)

var ErrValidation = errors.New("datos inválidos")

const (
	defaultPageLimit  = 50
	maxPageLimit      = 100
	followersPageSize = 100
)

// FollowerResolver consulta el grafo de seguimiento de Relationships.
type FollowerResolver interface {
	Followers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error)
}

// FeedService orquesta las operaciones del muro.
type FeedService struct {
	store     store.FeedStore
	followers FollowerResolver
	logger    *slog.Logger
	ttl       time.Duration
}

// New inyecta el store, el resolvedor de seguidores, el logger y la
// vigencia de cada muro. Si followers es nil solo se actualiza el muro
// del autor; si logger es nil se usa el logger por defecto de slog.
func New(s store.FeedStore, followers FollowerResolver, logger *slog.Logger, ttl time.Duration) *FeedService {
	if logger == nil {
		logger = slog.Default()
	}
	return &FeedService{store: s, followers: followers, logger: logger, ttl: ttl}
}

// RefreshFeed reparte un post recién creado: lo escribe en el muro de
// su autor y en el de sus seguidores. Si Relationships no responde no
// se propaga el error: se registra un warning y el post igual queda en
// el muro del autor (degradación elegante: que el grafo esté caído no
// puede impedir que quien publicó vea su propia publicación).
// Devuelve cuántos muros se actualizaron.
func (s *FeedService) RefreshFeed(ctx context.Context, authorID, postID string, createdAt int64) (int, error) {
	authorID = strings.TrimSpace(authorID)
	postID = strings.TrimSpace(postID)
	if authorID == "" {
		return 0, fmt.Errorf("%w: author_id es obligatorio", ErrValidation)
	}
	if postID == "" {
		return 0, fmt.Errorf("%w: post_id es obligatorio", ErrValidation)
	}
	if createdAt <= 0 {
		return 0, fmt.Errorf("%w: created_at debe ser una fecha en epoch", ErrValidation)
	}

	targets := s.resolveTargets(ctx, authorID)
	if err := s.store.Add(ctx, targets, postID, time.Unix(createdAt, 0), s.ttl); err != nil {
		return 0, err
	}
	return len(targets), nil
}

// GetUserFeed devuelve una página del muro de userID, de más reciente
// a más antiguo. Un usuario sin publicaciones no es un error: devuelve
// una página vacía con total 0.
func (s *FeedService) GetUserFeed(ctx context.Context, userID string, limit, offset int32) (domain.Page, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return domain.Page{}, fmt.Errorf("%w: user_id es obligatorio", ErrValidation)
	}
	l, o := normalizePage(limit, offset)
	recs, total, err := s.store.Page(ctx, userID, l, o)
	if err != nil {
		return domain.Page{}, err
	}

	page := domain.Page{
		Entries: make([]domain.Entry, 0, len(recs)),
		Total:   total,
	}
	for _, r := range recs {
		page.Entries = append(page.Entries, domain.Entry{
			UserID:    userID,
			PostID:    r.PostID,
			CreatedAt: r.CreatedAt,
		})
	}
	if len(page.Entries) > 0 {
		page.LastUpdated = page.Entries[0].CreatedAt.UTC().Format(time.RFC3339Nano)
	}
	return page, nil
}

// resolveTargets arma la lista de destinatarios del post: primero el
// autor, después sus seguidores sin duplicados. Recorre el grafo en
// páginas de followersPageSize; si Relationships falla en el camino se
// conserva lo ya reunido y se avisa por warning.
func (s *FeedService) resolveTargets(ctx context.Context, authorID string) []string {
	seen := map[string]bool{authorID: true}
	targets := []string{authorID}
	if s.followers == nil {
		return targets
	}
	for offset := int32(0); ; offset += followersPageSize {
		ids, total, err := s.followers.Followers(ctx, authorID, followersPageSize, offset)
		if err != nil {
			s.logger.Warn("no se pudieron consultar los seguidores; el post solo se reparte al autor",
				"author_id", authorID, "error", err)
			return targets
		}
		for _, id := range ids {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			targets = append(targets, id)
		}
		if len(ids) == 0 || offset+int32(len(ids)) >= total {
			return targets
		}
	}
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

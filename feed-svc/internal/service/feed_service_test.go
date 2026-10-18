package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Makx3/Kite/feed-svc/internal/store"
)

type fakeStore struct {
	mu     sync.Mutex
	muros  map[string][]store.EntryRecord
	addErr error
	ultimo string
}

func newFakeStore() *fakeStore {
	return &fakeStore{muros: map[string][]store.EntryRecord{}}
}

func (f *fakeStore) Add(ctx context.Context, userIDs []string, postID string, createdAt time.Time, ttl time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.addErr != nil {
		return f.addErr
	}
	f.ultimo = postID
	for _, uid := range userIDs {
		f.muros[uid] = append(f.muros[uid], store.EntryRecord{PostID: postID, CreatedAt: createdAt})
	}
	return nil
}

func (f *fakeStore) Page(ctx context.Context, userID string, limit, offset int32) ([]store.EntryRecord, int32, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	recs := f.muros[userID]
	total := int32(len(recs))
	if offset >= total {
		return []store.EntryRecord{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return recs[offset:end], total, nil
}

func (f *fakeStore) Ping(ctx context.Context) error { return nil }
func (f *fakeStore) Close() error                   { return nil }

func (f *fakeStore) muroDe(userID string) []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	ids := []string{}
	for _, r := range f.muros[userID] {
		ids = append(ids, r.PostID)
	}
	return ids
}

type fakeFollowers struct {
	all []string
	err error
}

func (f *fakeFollowers) Followers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	total := int32(len(f.all))
	if offset >= total {
		return []string{}, total, nil
	}
	end := offset + limit
	if end > total {
		end = total
	}
	ids := f.all[offset:end]
	if ids == nil {
		ids = []string{}
	}
	return ids, total, nil
}

func TestRefreshFeed_Validation(t *testing.T) {
	svc := New(newFakeStore(), nil, nil, 0)
	if _, err := svc.RefreshFeed(context.Background(), "  ", "p1", time.Now().Unix()); !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation sin autor")
	}
	if _, err := svc.RefreshFeed(context.Background(), "u1", " ", time.Now().Unix()); !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation sin post")
	}
	if _, err := svc.RefreshFeed(context.Background(), "u1", "p1", 0); !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation con created_at no válido")
	}
}

func TestRefreshFeed_AutorYSeguidoresSinDuplicados(t *testing.T) {
	fs := newFakeStore()
	resolver := &fakeFollowers{all: []string{"u1", "u2"}}
	svc := New(fs, resolver, nil, 0)

	n, err := svc.RefreshFeed(context.Background(), "u1", "p1", 1700000000)
	if err != nil {
		t.Fatal(err)
	}
	// u1 ya es autor: no debe duplicarse en la lista de destinatarios.
	if n != 2 {
		t.Fatalf("quería 2 muros actualizados y vino %d", n)
	}
	if got := fs.muroDe("u1"); len(got) != 1 {
		t.Fatalf("muro del autor: %v", got)
	}
	if got := fs.muroDe("u2"); len(got) != 1 {
		t.Fatalf("muro del seguidor: %v", got)
	}
}

func TestRefreshFeed_RecorreTodasLasPaginas(t *testing.T) {
	fs := newFakeStore()
	seguimientos := make([]string, 150) // dos páginas de a 100
	for i := range seguimientos {
		seguimientos[i] = fmt.Sprintf("f%03d", i)
	}
	svc := New(fs, &fakeFollowers{all: seguimientos}, nil, 0)

	n, err := svc.RefreshFeed(context.Background(), "autor", "p9", 1700000000)
	if err != nil {
		t.Fatal(err)
	}
	if n != 151 { // autor + los 150 seguidores
		t.Fatalf("quería 151 muros y vino %d", n)
	}
}

func TestRefreshFeed_GrafoCaidoSoloActualizaAlAutor(t *testing.T) {
	fs := newFakeStore()
	resolver := &fakeFollowers{err: errors.New("relationships caído")}
	svc := New(fs, resolver, nil, 0)

	n, err := svc.RefreshFeed(context.Background(), "u1", "p1", 1700000000)
	if err != nil {
		t.Fatalf("la degradación no debe devolver error: %v", err)
	}
	if n != 1 {
		t.Fatalf("quería solo el muro del autor y vino %d", n)
	}
	if got := fs.muroDe("u1"); len(got) != 1 {
		t.Fatalf("muro del autor: %v", got)
	}
}

func TestGetUserFeed_PaginacionPorDefecto(t *testing.T) {
	fs := newFakeStore()
	_ = fs.Add(context.Background(), []string{"u1"}, "p1", time.Now(), 0)
	svc := New(fs, nil, nil, 0)

	page, err := svc.GetUserFeed(context.Background(), "u1", 0, -5)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Entries) != 1 {
		t.Fatalf("total=%d entries=%d", page.Total, len(page.Entries))
	}
	if page.LastUpdated == "" {
		t.Fatal("LastUpdated vacío con un muro no vacío")
	}

	vacia, err := svc.GetUserFeed(context.Background(), "nadie", 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if vacia.Total != 0 || len(vacia.Entries) != 0 || vacia.LastUpdated != "" {
		t.Fatalf("página vacía mal formada: %+v", vacia)
	}
}

func TestGetUserFeed_Validation(t *testing.T) {
	svc := New(newFakeStore(), nil, nil, 0)
	if _, err := svc.GetUserFeed(context.Background(), strings.Repeat(" ", 3), 10, 0); !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation")
	}
}

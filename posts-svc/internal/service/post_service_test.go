package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Makx3/Kite/posts-svc/internal/store"
)

type fakeStore struct {
	recs []store.PostRecord
}

func (f *fakeStore) Create(ctx context.Context, userID, content string) (store.PostRecord, error) {
	now := time.Now()
	id := "id1"
	f.recs = append(f.recs, store.PostRecord{ID: id, UserID: userID, Content: content, CreatedAt: now})
	return f.recs[len(f.recs)-1], nil
}
func (f *fakeStore) Get(ctx context.Context, id string) (store.PostRecord, error) {
	for _, r := range f.recs {
		if r.ID == id {
			return r, nil
		}
	}
	return store.PostRecord{}, store.ErrNotFound
}
func (f *fakeStore) GetByIDs(ctx context.Context, ids []string) ([]store.PostRecord, error) {
	res := []store.PostRecord{}
	for _, r := range f.recs {
		for _, id := range ids {
			if r.ID == id {
				res = append(res, r)
			}
		}
	}
	return res, nil
}
func (f *fakeStore) GetByUser(ctx context.Context, userID string, limit, offset int32) ([]store.PostRecord, int32, error) {
	res := []store.PostRecord{}
	for _, r := range f.recs {
		if r.UserID == userID {
			res = append(res, r)
		}
	}
	return res, int32(len(res)), nil
}
func (f *fakeStore) Delete(ctx context.Context, id, userID string) error {
	for i, r := range f.recs {
		if r.ID == id && r.UserID == userID {
			f.recs = append(f.recs[:i], f.recs[i+1:]...)
			return nil
		}
	}
	return store.ErrNotFound
}
func (f *fakeStore) Ping(ctx context.Context) error { return nil }
func (f *fakeStore) Close() error                   { return nil }

type fakeFeed struct{}

func (f *fakeFeed) RefreshFeed(ctx context.Context, authorID, postID string, createdAt time.Time) error {
	return nil
}

func TestCreatePost_Validation(t *testing.T) {
	svc := New(&fakeStore{}, &fakeFeed{}, nil)
	_, err := svc.CreatePost(context.Background(), "u1", strings.Repeat("a", 281))
	if !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation para contenido muy largo")
	}
	_, err = svc.CreatePost(context.Background(), "u1", strings.Repeat("a", 280))
	if err != nil {
		t.Fatal(err)
	}
	_, err = svc.CreatePost(context.Background(), "u1", "   ")
	if !errors.Is(err, ErrValidation) {
		t.Fatal("esperaba ErrValidation para vacío")
	}
}

func TestGetPost_NotFound(t *testing.T) {
	svc := New(&fakeStore{}, nil, nil)
	_, err := svc.GetPost(context.Background(), "noexiste")
	if !errors.Is(err, store.ErrNotFound) {
		t.Fatal("esperaba ErrNotFound")
	}
}

func TestDeletePost_RequiresMatch(t *testing.T) {
	fs := &fakeStore{}
	svc := New(fs, nil, nil)
	p, err := svc.CreatePost(context.Background(), "u1", "hola")
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.DeletePost(context.Background(), p.ID, "u2"); !errors.Is(err, store.ErrNotFound) {
		t.Fatal("debería fallar si no coincide autor")
	}
	if err := svc.DeletePost(context.Background(), p.ID, "u1"); err != nil {
		t.Fatal(err)
	}
}

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/Makx3/Kite/likes-svc/internal/store"
)

// fakeStore guarda los likes en memoria y registra las llamadas.
type fakeStore struct {
	likes map[string]map[string]bool // postID -> userID -> existe
	seq   int
	err   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{likes: map[string]map[string]bool{}}
}

func (f *fakeStore) Create(_ context.Context, postID, userID string) (store.LikeRecord, bool, error) {
	if f.err != nil {
		return store.LikeRecord{}, false, f.err
	}
	if f.likes[postID] == nil {
		f.likes[postID] = map[string]bool{}
	}
	if f.likes[postID][userID] {
		f.seq++
		return store.LikeRecord{ID: fmt.Sprintf("like-%d", f.seq), PostID: postID, UserID: userID}, false, nil
	}
	f.likes[postID][userID] = true
	f.seq++
	return store.LikeRecord{ID: fmt.Sprintf("like-%d", f.seq), PostID: postID, UserID: userID}, true, nil
}

func (f *fakeStore) Delete(_ context.Context, postID, userID string) error {
	if f.err != nil {
		return f.err
	}
	if !f.likes[postID][userID] {
		return store.ErrNotFound
	}
	delete(f.likes[postID], userID)
	return nil
}

func (f *fakeStore) ListByPost(_ context.Context, postID string, _, _ int32) ([]store.LikeRecord, int32, error) {
	if f.err != nil {
		return nil, 0, f.err
	}
	var total int32
	for range f.likes[postID] {
		total++
	}
	return []store.LikeRecord{}, total, nil
}

func (f *fakeStore) Exists(_ context.Context, postID, userID string) (bool, error) {
	if f.err != nil {
		return false, f.err
	}
	return f.likes[postID][userID], nil
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close() error               { return nil }

// fakeNotifier cuenta las notificaciones enviadas y puede fallar.
type fakeNotifier struct {
	calls int
	err   error
}

func (n *fakeNotifier) NotifyPostLiked(context.Context, string, string, string) error {
	n.calls++
	return n.err
}

func newTestService() (*LikeService, *fakeStore, *fakeNotifier) {
	fs := newFakeStore()
	fn := &fakeNotifier{}
	return New(fs, fn, nil), fs, fn
}

func TestLikePostCreaYNotifica(t *testing.T) {
	svc, _, fn := newTestService()

	like, created, err := svc.LikePost(context.Background(), "p1", "u1", "author1")
	if err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	if !created {
		t.Error("created = false, se esperaba true")
	}
	if like.PostID != "p1" || like.UserID != "u1" || like.ID == "" {
		t.Errorf("like inesperado: %+v", like)
	}
	if fn.calls != 1 {
		t.Errorf("notificaciones = %d, se esperaba 1", fn.calls)
	}
}

func TestLikePostIdempotenteNoNotificaDosVeces(t *testing.T) {
	svc, _, fn := newTestService()

	if _, _, err := svc.LikePost(context.Background(), "p1", "u1", "author1"); err != nil {
		t.Fatalf("1a LikePost: %v", err)
	}
	like, created, err := svc.LikePost(context.Background(), "p1", "u1", "author1")
	if err != nil {
		t.Fatalf("2a LikePost: %v", err)
	}
	if created {
		t.Error("created = true en la repetición, se esperaba false")
	}
	if like.ID == "" {
		t.Error("la repetición debe devolver el like existente")
	}
	if fn.calls != 1 {
		t.Errorf("notificaciones = %d, se esperaba 1 (sin doble notificación)", fn.calls)
	}
}

func TestLikePostFallaDeNotificacionNoRompe(t *testing.T) {
	fs := newFakeStore()
	fn := &fakeNotifier{err: errors.New("notifications caído")}
	svc := New(fs, fn, nil)

	like, created, err := svc.LikePost(context.Background(), "p1", "u1", "author1")
	if err != nil {
		t.Fatalf("LikePost no debe fallar si Notifications cae: %v", err)
	}
	if !created || like.ID == "" {
		t.Error("el like debe quedar persistido igual")
	}
	if !fs.likes["p1"]["u1"] {
		t.Error("el like no quedó en el store")
	}
	if fn.calls != 1 {
		t.Errorf("notificaciones = %d, se esperaba 1", fn.calls)
	}
}

func TestLikePostValidacion(t *testing.T) {
	svc, _, _ := newTestService()
	casos := []struct {
		name, postID, userID, author string
	}{
		{"sin post", "", "u1", "a1"},
		{"sin user", "p1", "", "a1"},
		{"sin author", "p1", "u1", ""},
	}
	for _, c := range casos {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := svc.LikePost(context.Background(), c.postID, c.userID, c.author)
			if !errors.Is(err, ErrValidation) {
				t.Fatalf("err = %v, se esperaba ErrValidation", err)
			}
		})
	}
}

func TestLikePostStoreError(t *testing.T) {
	fs := newFakeStore()
	fs.err = errors.New("mongo caído")
	svc := New(fs, nil, nil)

	if _, _, err := svc.LikePost(context.Background(), "p1", "u1", "a1"); err == nil {
		t.Fatal("se esperaba error del store")
	}
}

func TestUnlikePost(t *testing.T) {
	svc, _, _ := newTestService()
	if _, _, err := svc.LikePost(context.Background(), "p1", "u1", "a1"); err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	if err := svc.UnlikePost(context.Background(), "p1", "u1"); err != nil {
		t.Fatalf("UnlikePost: %v", err)
	}
	if err := svc.UnlikePost(context.Background(), "p1", "u1"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("err = %v, se esperaba ErrNotFound", err)
	}
}

func TestGetPostLikesVacio(t *testing.T) {
	svc, _, _ := newTestService()
	ids, total, err := svc.GetPostLikes(context.Background(), "p-sin-likes", 0, 0)
	if err != nil {
		t.Fatalf("GetPostLikes: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, se esperaba 0", total)
	}
	if ids == nil || len(ids) != 0 {
		t.Errorf("ids = %#v, se esperaba lista no nil vacía", ids)
	}
}

func TestGetPostLikesListaAumentaTotal(t *testing.T) {
	svc, _, _ := newTestService()
	for i := 0; i < 3; i++ {
		if _, _, err := svc.LikePost(context.Background(), "p1", fmt.Sprintf("u%d", i), "a1"); err != nil {
			t.Fatalf("LikePost: %v", err)
		}
	}
	_, total, err := svc.GetPostLikes(context.Background(), "p1", 0, 0)
	if err != nil {
		t.Fatalf("GetPostLikes: %v", err)
	}
	if total != 3 {
		t.Errorf("total = %d, se esperaba 3", total)
	}
}

func TestHasLiked(t *testing.T) {
	svc, _, _ := newTestService()
	liked, err := svc.HasLiked(context.Background(), "p1", "u1")
	if err != nil || liked {
		t.Fatalf("liked = %v, err = %v; se esperaba false sin error", liked, err)
	}
	if _, _, err := svc.LikePost(context.Background(), "p1", "u1", "a1"); err != nil {
		t.Fatalf("LikePost: %v", err)
	}
	liked, err = svc.HasLiked(context.Background(), "p1", "u1")
	if err != nil || !liked {
		t.Fatalf("liked = %v, err = %v; se esperaba true sin error", liked, err)
	}
}

func TestNormalizePage(t *testing.T) {
	casos := []struct {
		limit, offset, wantL, wantO int32
	}{
		{0, 0, defaultPageLimit, 0},
		{-5, -1, defaultPageLimit, 0},
		{1000, 10, defaultPageLimit, 10},
		{20, 0, 20, 0},
	}
	for _, c := range casos {
		l, o := normalizePage(c.limit, c.offset)
		if l != c.wantL || o != c.wantO {
			t.Errorf("normalizePage(%d, %d) = (%d, %d), se esperaba (%d, %d)",
				c.limit, c.offset, l, o, c.wantL, c.wantO)
		}
	}
}

package service

import (
	"context"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/Makx3/Kite/relationships-svc/internal/store"
)

// fakeStore es un doble de RelationshipStore en memoria para aislar
// las pruebas de la lógica de negocio de PostgreSQL.
type fakeStore struct {
	follows  map[string]store.FollowRecord // clave "follower|followee"
	deleteEr error
	createEr error
}

func newFakeStore() *fakeStore {
	return &fakeStore{follows: make(map[string]store.FollowRecord)}
}

func key(a, b string) string { return a + "|" + b }

// Create es idempotente e informa created=true solo la primera vez.
func (f *fakeStore) Create(_ context.Context, followerID, followeeID string) (store.FollowRecord, bool, error) {
	if f.createEr != nil {
		return store.FollowRecord{}, false, f.createEr
	}
	k := key(followerID, followeeID)
	if rec, ok := f.follows[k]; ok {
		return rec, false, nil
	}
	rec := store.FollowRecord{FollowerID: followerID, FolloweeID: followeeID, CreatedAt: time.Now()}
	f.follows[k] = rec
	return rec, true, nil
}

func (f *fakeStore) Delete(_ context.Context, followerID, followeeID string) error {
	if f.deleteEr != nil {
		return f.deleteEr
	}
	k := key(followerID, followeeID)
	if _, ok := f.follows[k]; !ok {
		return store.ErrNotFound
	}
	delete(f.follows, k)
	return nil
}

func (f *fakeStore) ListFollowers(_ context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	all := make([]string, 0)
	for _, rec := range f.follows {
		if rec.FolloweeID == userID {
			all = append(all, rec.FollowerID)
		}
	}
	sort.Strings(all)
	return page(all, limit, offset), int32(len(all)), nil
}

func (f *fakeStore) ListFollowing(_ context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	all := make([]string, 0)
	for _, rec := range f.follows {
		if rec.FollowerID == userID {
			all = append(all, rec.FolloweeID)
		}
	}
	sort.Strings(all)
	return page(all, limit, offset), int32(len(all)), nil
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close()                     {}

// page aplica limit/offset a una lista ya ordenada, imitando al store real.
func page(ids []string, limit, offset int32) []string {
	if offset >= int32(len(ids)) {
		return []string{}
	}
	end := offset + limit
	if end > int32(len(ids)) {
		end = int32(len(ids))
	}
	return ids[offset:end]
}

// fakeNotifier cuenta los disparos y permite simular un Notifications caído.
type fakeNotifier struct {
	calls int
	err   error
}

func (f *fakeNotifier) NotifyNewFollower(context.Context, string, string) error {
	f.calls++
	return f.err
}

func TestFollowUserOK(t *testing.T) {
	notifier := &fakeNotifier{}
	svc := New(newFakeStore(), notifier, nil)

	follow, err := svc.FollowUser(context.Background(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("FollowUser: %v", err)
	}
	if follow.FollowerID != "u-1" || follow.FolloweeID != "u-2" {
		t.Errorf("relación inesperada: %+v", follow)
	}
	if follow.CreatedAt.IsZero() {
		t.Error("created_at vacío")
	}
	if notifier.calls != 1 {
		t.Errorf("se esperaba 1 notificación, hubo %d", notifier.calls)
	}
}

func TestFollowUserIdempotenteNoRenotifica(t *testing.T) {
	notifier := &fakeNotifier{}
	svc := New(newFakeStore(), notifier, nil)

	primero, err := svc.FollowUser(context.Background(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("primer FollowUser: %v", err)
	}
	segundo, err := svc.FollowUser(context.Background(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("segundo FollowUser devolvió error: %v", err)
	}
	if !segundo.CreatedAt.Equal(primero.CreatedAt) {
		t.Error("seguir dos veces no debe crear una relación nueva")
	}
	if notifier.calls != 1 {
		t.Errorf("seguir dos veces no debe renotificar: hubo %d notificaciones", notifier.calls)
	}
}

func TestFollowUserDegradaSiNotificationsFalla(t *testing.T) {
	notifier := &fakeNotifier{err: errors.New("notifications no disponible")}
	svc := New(newFakeStore(), notifier, nil)

	// El seguimiento debe completarse aunque la alerta falle.
	follow, err := svc.FollowUser(context.Background(), "u-1", "u-2")
	if err != nil {
		t.Fatalf("FollowUser no debe fallar si Notifications cae: %v", err)
	}
	if follow.FollowerID != "u-1" || follow.FolloweeID != "u-2" {
		t.Errorf("relación inesperada: %+v", follow)
	}
	if notifier.calls != 1 {
		t.Errorf("se esperaba 1 intento de notificación, hubo %d", notifier.calls)
	}
}

func TestFollowUserValidacion(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)
	casos := []struct {
		nombre   string
		follower string
		followee string
	}{
		{"follower vacío", "", "u-2"},
		{"followee vacío", "u-1", "   "},
		{"seguirse a sí mismo", "u-1", "u-1"},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			if _, err := svc.FollowUser(context.Background(), c.follower, c.followee); !errors.Is(err, ErrValidation) {
				t.Errorf("esperaba ErrValidation, obtuve: %v", err)
			}
		})
	}
}

func TestUnfollowUser(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)
	if _, err := svc.FollowUser(context.Background(), "u-1", "u-2"); err != nil {
		t.Fatalf("FollowUser: %v", err)
	}

	if err := svc.UnfollowUser(context.Background(), "u-1", "u-2"); err != nil {
		t.Fatalf("UnfollowUser: %v", err)
	}
	// Dejar de seguir algo que ya no se sigue: error ordenado.
	if err := svc.UnfollowUser(context.Background(), "u-1", "u-2"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperaba ErrNotFound, obtuve: %v", err)
	}
}

func TestGetFollowersYFollowing(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)

	// u-1 y u-2 siguen a u-3; u-1 sigue a u-2.
	for _, p := range [][2]string{{"u-1", "u-3"}, {"u-2", "u-3"}, {"u-1", "u-2"}} {
		if _, err := svc.FollowUser(context.Background(), p[0], p[1]); err != nil {
			t.Fatalf("FollowUser %v: %v", p, err)
		}
	}

	seguidores, total, err := svc.GetFollowers(context.Background(), "u-3", 0, 0)
	if err != nil {
		t.Fatalf("GetFollowers: %v", err)
	}
	if len(seguidores) != 2 || total != 2 {
		t.Errorf("u-3 debería tener 2 seguidores (total 2), obtuve %v total=%d", seguidores, total)
	}

	siguiendo, total, err := svc.GetFollowing(context.Background(), "u-1", 0, 0)
	if err != nil {
		t.Fatalf("GetFollowing: %v", err)
	}
	if len(siguiendo) != 2 || total != 2 {
		t.Errorf("u-1 debería seguir a 2 (total 2), obtuve %v total=%d", siguiendo, total)
	}
}

func TestGetFollowersPaginacion(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)

	// u-1, u-2 y u-3 siguen a u-4.
	for _, f := range []string{"u-1", "u-2", "u-3"} {
		if _, err := svc.FollowUser(context.Background(), f, "u-4"); err != nil {
			t.Fatalf("FollowUser %s: %v", f, err)
		}
	}

	primera, total, err := svc.GetFollowers(context.Background(), "u-4", 2, 0)
	if err != nil {
		t.Fatalf("GetFollowers página 1: %v", err)
	}
	if len(primera) != 2 || total != 3 {
		t.Errorf("página 1: esperaba 2 ids y total 3, obtuve %v total=%d", primera, total)
	}

	segunda, total, err := svc.GetFollowers(context.Background(), "u-4", 2, 2)
	if err != nil {
		t.Fatalf("GetFollowers página 2: %v", err)
	}
	if len(segunda) != 1 || total != 3 {
		t.Errorf("página 2: esperaba 1 id y total 3, obtuve %v total=%d", segunda, total)
	}

	// Fuera de rango: lista vacía pero el total sigue siendo el real.
	vacia, total, err := svc.GetFollowers(context.Background(), "u-4", 2, 99)
	if err != nil {
		t.Fatalf("GetFollowers fuera de rango: %v", err)
	}
	if len(vacia) != 0 || total != 3 {
		t.Errorf("fuera de rango: esperaba 0 ids y total 3, obtuve %v total=%d", vacia, total)
	}
}

func TestPaginacionInvalida(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)

	if _, _, err := svc.GetFollowers(context.Background(), "u-1", -1, 0); !errors.Is(err, ErrValidation) {
		t.Errorf("limit negativo: esperaba ErrValidation, obtuve %v", err)
	}
	if _, _, err := svc.GetFollowers(context.Background(), "u-1", 0, -1); !errors.Is(err, ErrValidation) {
		t.Errorf("offset negativo: esperaba ErrValidation, obtuve %v", err)
	}
}

func TestGetFollowersValidacion(t *testing.T) {
	svc := New(newFakeStore(), nil, nil)

	if _, _, err := svc.GetFollowers(context.Background(), "  ", 0, 0); !errors.Is(err, ErrValidation) {
		t.Errorf("esperaba ErrValidation, obtuve: %v", err)
	}
}

func TestNormalizePage(t *testing.T) {
	casos := []struct {
		limit, offset         int32
		wantLimit, wantOffset int32
	}{
		{0, 0, defaultPageLimit, 0},
		{10, 5, 10, 5},
		{maxPageLimit + 50, 0, maxPageLimit, 0},
	}
	for _, c := range casos {
		gotLimit, gotOffset, err := normalizePage(c.limit, c.offset)
		if err != nil {
			t.Fatalf("normalizePage(%d,%d): %v", c.limit, c.offset, err)
		}
		if gotLimit != c.wantLimit || gotOffset != c.wantOffset {
			t.Errorf("normalizePage(%d,%d) = (%d,%d), esperaba (%d,%d)",
				c.limit, c.offset, gotLimit, gotOffset, c.wantLimit, c.wantOffset)
		}
	}
}

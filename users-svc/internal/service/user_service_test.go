package service

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/Makx3/Kite/users-svc/internal/store"
)

// fakeStore es un doble de UserStore en memoria para aislar las
// pruebas de la lógica de negocio de PostgreSQL.
type fakeStore struct {
	users    map[string]store.UserRecord
	nextID   int
	createEr error // error inyectable para simular fallos de BD
	getErr   error
}

func newFakeStore() *fakeStore {
	return &fakeStore{users: make(map[string]store.UserRecord)}
}

func (f *fakeStore) Create(_ context.Context, username, email string) (store.UserRecord, error) {
	if f.createEr != nil {
		return store.UserRecord{}, f.createEr
	}
	f.nextID++
	rec := store.UserRecord{
		ID:        "usr-" + strings.Repeat("0", 3-len(itoa(f.nextID))) + itoa(f.nextID),
		Username:  username,
		Email:     email,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	f.users[rec.ID] = rec
	return rec, nil
}

func (f *fakeStore) Get(_ context.Context, id string) (store.UserRecord, error) {
	if f.getErr != nil {
		return store.UserRecord{}, f.getErr
	}
	rec, ok := f.users[id]
	if !ok {
		return store.UserRecord{}, store.ErrNotFound
	}
	return rec, nil
}

func (f *fakeStore) Update(_ context.Context, id string, username, email *string) (store.UserRecord, error) {
	rec, ok := f.users[id]
	if !ok {
		return store.UserRecord{}, store.ErrNotFound
	}
	if username != nil {
		rec.Username = *username
	}
	if email != nil {
		rec.Email = *email
	}
	rec.UpdatedAt = time.Now()
	f.users[id] = rec
	return rec, nil
}

func (f *fakeStore) Ping(context.Context) error { return nil }
func (f *fakeStore) Close()                     {}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// strPtr es un atajo para los campos opcionales del parche.
func strPtr(s string) *string { return &s }

func TestCreateUserOK(t *testing.T) {
	svc := New(newFakeStore())

	user, err := svc.CreateUser(context.Background(), "  msepulveda  ", "max@ufro.cl")
	if err != nil {
		t.Fatalf("CreateUser devolvió error: %v", err)
	}
	if user.Username != "msepulveda" {
		t.Errorf("username no fue normalizado: %q", user.Username)
	}
	if user.Email != "max@ufro.cl" {
		t.Errorf("email no fue normalizado: %q", user.Email)
	}
	if user.ID == "" {
		t.Error("ID vacío")
	}
}

func TestCreateUserValidacion(t *testing.T) {
	svc := New(newFakeStore())
	casos := []struct {
		nombre   string
		username string
		email    string
		wantErr  bool
	}{
		{"username muy corto", "ab", "a@b.cl", true},
		{"username muy largo", strings.Repeat("x", 33), "a@b.cl", true},
		{"email sin arroba", "usuario", "nofuera", true},
		{"email vacío", "usuario", "", true},
		{"válido", "usuario1", "u@ufro.cl", false},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			_, err := svc.CreateUser(context.Background(), c.username, c.email)
			if c.wantErr && !errors.Is(err, ErrValidation) {
				t.Errorf("esperaba ErrValidation, obtuve: %v", err)
			}
			if !c.wantErr && err != nil {
				t.Errorf("esperaba éxito, obtuve: %v", err)
			}
		})
	}
}

func TestGetUserNoEncontrado(t *testing.T) {
	svc := New(newFakeStore())

	_, err := svc.GetUser(context.Background(), "usr-999")
	if !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperaba ErrNotFound, obtuve: %v", err)
	}
}

func TestGetUserVacio(t *testing.T) {
	svc := New(newFakeStore())

	_, err := svc.GetUser(context.Background(), "   ")
	if !errors.Is(err, ErrValidation) {
		t.Errorf("esperaba ErrValidation, obtuve: %v", err)
	}
}

func TestUpdateUserParche(t *testing.T) {
	svc := New(newFakeStore())
	creado, err := svc.CreateUser(context.Background(), "diego", "diego@ufro.cl")
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	// Solo email: username debe conservarse.
	actualizado, err := svc.UpdateUser(context.Background(), creado.ID, nil, strPtr("diego.nuevo@ufro.cl"))
	if err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}
	if actualizado.Username != "diego" {
		t.Errorf("username cambió sin que se pidiera: %q", actualizado.Username)
	}
	if actualizado.Email != "diego.nuevo@ufro.cl" {
		t.Errorf("email no se actualizó: %q", actualizado.Email)
	}
}

func TestUpdateUserSinCampos(t *testing.T) {
	svc := New(newFakeStore())

	_, err := svc.UpdateUser(context.Background(), "usr-1", nil, nil)
	if !errors.Is(err, ErrValidation) {
		t.Errorf("esperaba ErrValidation, obtuve: %v", err)
	}
}

func TestUpdateUserEmailInvalido(t *testing.T) {
	svc := New(newFakeStore())
	creado, _ := svc.CreateUser(context.Background(), "diego", "diego@ufro.cl")

	_, err := svc.UpdateUser(context.Background(), creado.ID, nil, strPtr("esto-no-es-email"))
	if !errors.Is(err, ErrValidation) {
		t.Errorf("esperaba ErrValidation, obtuve: %v", err)
	}
}

func TestErroresDeStoreSePropagan(t *testing.T) {
	f := newFakeStore()
	f.createEr = store.ErrEmailTaken
	svc := New(f)

	_, err := svc.CreateUser(context.Background(), "usuario", "u@ufro.cl")
	if !errors.Is(err, store.ErrEmailTaken) {
		t.Errorf("esperaba que ErrEmailTaken se propagara, obtuve: %v", err)
	}
}

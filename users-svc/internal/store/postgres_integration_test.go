package store_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/Makx3/Kite/users-svc/internal/store"
)

// TestPostgresIntegracion valida la persistencia contra una base de
// datos real. Se salta si USERS_TEST_DB_DSN no está definida, para
// que `go test ./...` siga siendo ejecutable sin infraestructura.
//
// Para correrlo:
//
//	docker run -d --name kite-pg-test -e POSTGRES_USER=kite \
//	  -e POSTGRES_PASSWORD=kite123 -e POSTGRES_DB=users_test \
//	  -p 55432:5432 postgres:16
//	USERS_TEST_DB_DSN="postgres://kite:kite123@localhost:55432/users_test?sslmode=disable" \
//	  go test ./internal/store/ -v
func TestPostgresIntegracion(t *testing.T) {
	dsn := os.Getenv("USERS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("USERS_TEST_DB_DSN no definida: se omite la prueba de integración")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pg, err := store.NewPostgres(ctx, dsn, 5)
	if err != nil {
		t.Fatalf("NewPostgres: %v", err)
	}
	defer pg.Close()

	if err := pg.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// username y email únicos por corrida para no chocar con datos
	// previos: ahora ambos tienen restricción UNIQUE.
	unique := fmt.Sprintf("%d", time.Now().UnixNano())
	username := "it" + unique
	email := "it-" + unique + "@ufro.cl"
	displayName := "Integración " + unique

	creado, err := pg.Create(ctx, username, email, displayName)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if creado.ID == "" {
		t.Fatal("Create no devolvió user_id")
	}
	if creado.DisplayName != displayName {
		t.Errorf("Create devolvió display_name %q, esperaba %q", creado.DisplayName, displayName)
	}
	if creado.CreatedAt.IsZero() {
		t.Error("created_at sin valor (la BD debe generarlo)")
	}

	// Email repetido: la restricción UNIQUE debe traducirse.
	if _, err := pg.Create(ctx, username+"x", email, displayName); !errors.Is(err, store.ErrEmailTaken) {
		t.Errorf("esperaba ErrEmailTaken, obtuve: %v", err)
	}

	// Username repetido: se distingue del email gracias al nombre
	// de la restricción violada.
	if _, err := pg.Create(ctx, username, "otro-"+email, displayName); !errors.Is(err, store.ErrUsernameTaken) {
		t.Errorf("esperaba ErrUsernameTaken, obtuve: %v", err)
	}

	// Get existente e inexistente (incluido un UUID malformado).
	obtenido, err := pg.Get(ctx, creado.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if obtenido.Email != email || obtenido.DisplayName != displayName {
		t.Errorf("Get devolvió %q/%q, esperaba %q/%q", obtenido.Email, obtenido.DisplayName, email, displayName)
	}
	if _, err := pg.Get(ctx, "no-es-uuid"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperaba ErrNotFound para id malformado, obtuve: %v", err)
	}

	// Update de parche: solo display_name; username y email intactos.
	nuevoNombre := displayName + " (editado)"
	actualizado, err := pg.Update(ctx, creado.ID, &nuevoNombre, nil)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if actualizado.DisplayName != nuevoNombre {
		t.Errorf("display_name no actualizado: %q", actualizado.DisplayName)
	}
	if actualizado.Username != username {
		t.Errorf("username cambió sin pedirlo: %q", actualizado.Username)
	}
	if actualizado.Email != email {
		t.Errorf("email cambió sin pedirlo: %q", actualizado.Email)
	}
	if !actualizado.UpdatedAt.After(creado.UpdatedAt) {
		t.Error("updated_at no avanzó")
	}

	// Update de parche: solo email; display_name intacto.
	nuevoEmail := "editado-" + email
	soloEmail, err := pg.Update(ctx, creado.ID, nil, &nuevoEmail)
	if err != nil {
		t.Fatalf("Update (email): %v", err)
	}
	if soloEmail.Email != nuevoEmail {
		t.Errorf("email no actualizado: %q", soloEmail.Email)
	}
	if soloEmail.DisplayName != nuevoNombre {
		t.Errorf("display_name cambió sin pedirlo: %q", soloEmail.DisplayName)
	}

	// Update sobre id inexistente.
	if _, err := pg.Update(ctx, "3f9c1c1e-0000-4000-8000-000000000000", &nuevoNombre, nil); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperaba ErrNotFound, obtuve: %v", err)
	}

	// Ping (usado por el health check).
	if err := pg.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

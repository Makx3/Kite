package store_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/Makx3/Kite/relationships-svc/internal/store"
)

// TestPostgresIntegracion valida la persistencia contra una base de
// datos real. Se salta si RELATIONSHIPS_TEST_DB_DSN no está definida,
// para que `go test ./...` siga siendo ejecutable sin infraestructura.
//
// Para correrlo:
//
//	docker run -d --name kite-pg-rel -e POSTGRES_USER=kite \
//	  -e POSTGRES_PASSWORD=kite123 -e POSTGRES_DB=relationships_test \
//	  -p 55433:5432 postgres:16
//	RELATIONSHIPS_TEST_DB_DSN="postgres://kite:kite123@localhost:55433/relationships_test?sslmode=disable" \
//	  go test ./internal/store/ -v
func TestPostgresIntegracion(t *testing.T) {
	dsn := os.Getenv("RELATIONSHIPS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("RELATIONSHIPS_TEST_DB_DSN no definida: se omite la prueba de integración")
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

	// UUIDs válidos y únicos por corrida para no chocar con datos previos.
	a := "11111111-1111-4111-8111-111111111111"
	b := "22222222-2222-4222-8222-222222222222"
	c := "33333333-3333-4333-8333-333333333333"

	// Aislamiento entre corridas: limpiar las relaciones de prueba.
	for _, p := range [][2]string{{a, b}, {a, c}, {b, c}} {
		_ = pg.Delete(ctx, p[0], p[1])
	}

	rec, created, err := pg.Create(ctx, a, b)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if !created {
		t.Error("la primera inserción debe reportar created=true")
	}
	if rec.CreatedAt.IsZero() {
		t.Error("created_at sin valor (la BD debe generarlo)")
	}

	// Idempotencia: repetir no crea una fila nueva ni falla, y no se
	// reporta como creada (para no renotificar el mismo seguimiento).
	rec2, created2, err := pg.Create(ctx, a, b)
	if err != nil {
		t.Fatalf("Create idempotente: %v", err)
	}
	if created2 {
		t.Error("la repetición no debe reportar created=true")
	}
	if !rec2.CreatedAt.Equal(rec.CreatedAt) {
		t.Error("Create idempotente devolvió una fila distinta")
	}

	// Seguirse a sí mismo: la regla vive en el servicio y en la BD.
	if _, _, err := pg.Create(ctx, a, a); !errors.Is(err, store.ErrInvalidID) {
		t.Errorf("esperaba ErrInvalidID (check_violation), obtuve: %v", err)
	}

	// UUID malformado.
	if _, _, err := pg.Create(ctx, "no-es-uuid", b); !errors.Is(err, store.ErrInvalidID) {
		t.Errorf("esperaba ErrInvalidID, obtuve: %v", err)
	}

	if _, _, err := pg.Create(ctx, b, c); err != nil {
		t.Fatalf("Create b->c: %v", err)
	}

	seguidores, total, err := pg.ListFollowers(ctx, c, 10, 0)
	if err != nil {
		t.Fatalf("ListFollowers: %v", err)
	}
	if len(seguidores) != 1 || seguidores[0] != b || total != 1 {
		t.Errorf("ListFollowers(c) = %v total=%d, esperaba [%s] total=1", seguidores, total, b)
	}

	siguiendo, total, err := pg.ListFollowing(ctx, a, 10, 0)
	if err != nil {
		t.Fatalf("ListFollowing: %v", err)
	}
	if len(siguiendo) != 1 || siguiendo[0] != b || total != 1 {
		t.Errorf("ListFollowing(a) = %v total=%d, esperaba [%s] total=1", siguiendo, total, b)
	}

	// Paginación: con limit=1 y offset=1 no hay segunda página, pero el
	// total sigue reflejando la realidad.
	_, total, err = pg.ListFollowers(ctx, c, 1, 1)
	if err != nil {
		t.Fatalf("ListFollowers paginado: %v", err)
	}
	if total != 1 {
		t.Errorf("el total debe ser independiente de la página: obtuve %d", total)
	}

	// Delete existente y no existente.
	if err := pg.Delete(ctx, a, b); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := pg.Delete(ctx, a, b); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("esperaba ErrNotFound, obtuve: %v", err)
	}

	if err := pg.Ping(ctx); err != nil {
		t.Errorf("Ping: %v", err)
	}
}

//go:build integration

package store

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

func TestMongoIntegration(t *testing.T) {
	dsn := os.Getenv("POSTS_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("POSTS_TEST_DB_DSN no está definida; se omite test de integración")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	m, err := NewMongo(ctx, dsn)
	if err != nil {
		t.Fatalf("NewMongo: %v", err)
	}
	defer m.Close()
	if err := m.Migrate(ctx); err != nil {
		t.Fatalf("Migrate: %v", err)
	}

	// limpieza
	_, _ = m.db.Collection(collectionName).DeleteMany(ctx, bson.M{})

	rec, err := m.Create(ctx, "u1", "hola mundo")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if rec.ID == "" {
		t.Fatal("ID vacío")
	}

	got, err := m.Get(ctx, rec.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Content != "hola mundo" {
		t.Fatal("contenido distinto")
	}
	// El driver devuelve bson.DateTime; si mapRecord no lo convierte,
	// created_at queda en cero y la API publica fechas vacías.
	if got.CreatedAt.IsZero() {
		t.Fatal("created_at llegó en cero: bson.DateTime no se convirtió a time.Time")
	}

	list, total, err := m.GetByUser(ctx, "u1", 10, 0)
	if err != nil || total != 1 || len(list) != 1 {
		t.Fatalf("GetByUser: %v total=%d len=%d", err, total, len(list))
	}

	if err := m.Delete(ctx, rec.ID, "otro"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete con otro autor: quería ErrNotFound y vino %v", err)
	}
	if err := m.Delete(ctx, rec.ID, "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
}

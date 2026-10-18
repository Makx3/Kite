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
	dsn := os.Getenv("LIKES_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("LIKES_TEST_DB_DSN no está definida; se omite test de integración")
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

	_, _ = m.likes.DeleteMany(ctx, bson.M{})

	// alta normal
	rec, created, err := m.Create(ctx, "post1", "u1")
	if err != nil || !created {
		t.Fatalf("Create: created=%v err=%v", created, err)
	}
	if rec.ID == "" || rec.PostID != "post1" || rec.UserID != "u1" {
		t.Fatalf("registro inesperado: %+v", rec)
	}

	// idempotencia por índice único (post_id, user_id)
	rep, created, err := m.Create(ctx, "post1", "u1")
	if err != nil {
		t.Fatalf("Create repetido: %v", err)
	}
	if created {
		t.Fatal("la repetición no debe reportar created=true")
	}
	if rep.ID != rec.ID {
		t.Fatalf("la repetición debe devolver el like existente: %s != %s", rep.ID, rec.ID)
	}

	// otro like en el mismo post
	if _, _, err := m.Create(ctx, "post1", "u2"); err != nil {
		t.Fatalf("Create u2: %v", err)
	}

	// Exists
	if ok, err := m.Exists(ctx, "post1", "u1"); err != nil || !ok {
		t.Fatalf("Exists u1: ok=%v err=%v", ok, err)
	}
	if ok, err := m.Exists(ctx, "post1", "nobody"); err != nil || ok {
		t.Fatalf("Exists nobody: ok=%v err=%v", ok, err)
	}

	// paginación con total
	list, total, err := m.ListByPost(ctx, "post1", 1, 0)
	if err != nil || total != 2 || len(list) != 1 {
		t.Fatalf("ListByPost: %v total=%d len=%d", err, total, len(list))
	}
	if list[0].UserID != "u1" {
		t.Fatalf("orden inesperado: primer user_id = %s", list[0].UserID)
	}

	// borrar
	if err := m.Delete(ctx, "post1", "u1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if err := m.Delete(ctx, "post1", "u1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Delete repetido: quería ErrNotFound y vino %v", err)
	}
	_, total, err = m.ListByPost(ctx, "post1", 10, 0)
	if err != nil || total != 1 {
		t.Fatalf("total tras borrar: %v total=%d", err, total)
	}
}

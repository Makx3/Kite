//go:build integration

package store

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestRedisIntegration(t *testing.T) {
	dsn := os.Getenv("FEED_TEST_DB_DSN")
	if dsn == "" {
		t.Skip("FEED_TEST_DB_DSN no está definida; se omite test de integración")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rdb, err := NewRedis(ctx, dsn)
	if err != nil {
		t.Fatalf("NewRedis: %v", err)
	}
	defer rdb.Close()

	// limpieza de los muros que usa el test
	for _, uid := range []string{"t1", "t2", "t3"} {
		_ = rdb.rdb.Del(ctx, keyPrefix+uid).Err()
	}

	createdAt := time.Unix(1700000000, 0).UTC()
	if err := rdb.Add(ctx, []string{"t1", "t2"}, "p1", createdAt, 0); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if err := rdb.Add(ctx, []string{"t1"}, "p2", createdAt.Add(time.Minute), 0); err != nil {
		t.Fatalf("Add: %v", err)
	}

	// p2 es más reciente: encabeza el muro.
	recs, total, err := rdb.Page(ctx, "t1", 10, 0)
	if err != nil {
		t.Fatalf("Page: %v", err)
	}
	if total != 2 || len(recs) != 2 {
		t.Fatalf("total=%d len=%d", total, len(recs))
	}
	if recs[0].PostID != "p2" || recs[1].PostID != "p1" {
		t.Fatalf("orden incorrecto: %v", []string{recs[0].PostID, recs[1].PostID})
	}
	if !recs[0].CreatedAt.Equal(createdAt.Add(time.Minute)) {
		t.Fatalf("CreatedAt: %v", recs[0].CreatedAt)
	}

	// paginación: una entrada por página
	p1, _, err := rdb.Page(ctx, "t1", 1, 0)
	if err != nil || len(p1) != 1 || p1[0].PostID != "p2" {
		t.Fatalf("página 1: %v %v", p1, err)
	}
	p2, _, err := rdb.Page(ctx, "t1", 1, 1)
	if err != nil || len(p2) != 1 || p2[0].PostID != "p1" {
		t.Fatalf("página 2: %v %v", p2, err)
	}

	// muro inexistente: página vacía, total 0, sin error.
	vacia, total, err := rdb.Page(ctx, "t3", 10, 0)
	if err != nil || total != 0 || len(vacia) != 0 {
		t.Fatalf("muro inexistente: total=%d len=%d err=%v", total, len(vacia), err)
	}

	// TTL: con expiración la clave desaparece.
	if err := rdb.Add(ctx, []string{"t3"}, "p9", createdAt, time.Second); err != nil {
		t.Fatalf("Add con ttl: %v", err)
	}
	ttl, err := rdb.rdb.TTL(ctx, keyPrefix+"t3").Result()
	if err != nil || ttl <= 0 {
		t.Fatalf("ttl=%v err=%v", ttl, err)
	}
}

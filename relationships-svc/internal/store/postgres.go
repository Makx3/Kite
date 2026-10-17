package store

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// migrationsFS empaqueta los scripts SQL para que el binario sea
// autónomo (no se necesitan archivos sueltos en la imagen Docker).
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Postgres implementa RelationshipStore sobre un pool de pgx.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres conecta y valida el pool contra la base de datos. El
// reintento al arrancar es responsabilidad del caller (cmd/server).
func NewPostgres(ctx context.Context, dsn string, maxConns int32) (*Postgres, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("store: dsn inválida: %w", err)
	}
	cfg.MaxConns = maxConns

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("store: no se pudo crear el pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("store: ping falló: %w", err)
	}
	return &Postgres{pool: pool}, nil
}

// Migrate aplica los scripts embebidos en orden lexicográfico; son
// idempotentes, por lo que ejecutarlos en cada arranque es seguro.
func (p *Postgres) Migrate(ctx context.Context) error {
	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("store: leer migraciones: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Strings(names)

	for _, name := range names {
		sql, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("store: leer %s: %w", name, err)
		}
		if _, err := p.pool.Exec(ctx, string(sql)); err != nil {
			return fmt.Errorf("store: aplicar %s: %w", name, err)
		}
	}
	return nil
}

// Create registra el seguimiento de forma idempotente. Devuelve
// created=true solo cuando la arista es nueva: INSERT ... ON CONFLICT DO
// NOTHING no devuelve fila si ya existía, y en ese caso se recupera la
// fila previa para reportar el created_at original.
func (p *Postgres) Create(ctx context.Context, followerID, followeeID string) (FollowRecord, bool, error) {
	const insertQ = `
		INSERT INTO follows (follower_id, followee_id)
		VALUES ($1, $2)
		ON CONFLICT (follower_id, followee_id) DO NOTHING
		RETURNING follower_id::text, followee_id::text, created_at`

	var rec FollowRecord
	err := p.pool.QueryRow(ctx, insertQ, followerID, followeeID).Scan(
		&rec.FollowerID, &rec.FolloweeID, &rec.CreatedAt,
	)
	if err == nil {
		return rec, true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return FollowRecord{}, false, translate(err)
	}

	// La relación ya existía: devolverla tal cual, sin crear nada nuevo.
	const selectQ = `
		SELECT follower_id::text, followee_id::text, created_at
		FROM follows WHERE follower_id = $1 AND followee_id = $2`
	err = p.pool.QueryRow(ctx, selectQ, followerID, followeeID).Scan(
		&rec.FollowerID, &rec.FolloweeID, &rec.CreatedAt,
	)
	if err != nil {
		return FollowRecord{}, false, translate(err)
	}
	return rec, false, nil
}

// Delete elimina la relación y reporta ErrNotFound si no existía
// (fila no afectada), de modo que el caller reciba un error ordenado.
func (p *Postgres) Delete(ctx context.Context, followerID, followeeID string) error {
	const q = `DELETE FROM follows WHERE follower_id = $1 AND followee_id = $2`

	tag, err := p.pool.Exec(ctx, q, followerID, followeeID)
	if err != nil {
		return translate(err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// ListFollowers retorna una página de seguidores de userID (los user_id
// que lo siguen) más el total. El ORDER BY incluye la clave para que la
// paginación sea estable entre páginas con el mismo created_at.
func (p *Postgres) ListFollowers(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	const listQ = `SELECT follower_id::text FROM follows
		WHERE followee_id = $1 ORDER BY created_at, follower_id LIMIT $2 OFFSET $3`
	const countQ = `SELECT count(*) FROM follows WHERE followee_id = $1`
	return p.pageIDs(ctx, listQ, countQ, userID, limit, offset)
}

// ListFollowing retorna una página de a quiénes sigue userID más el total.
func (p *Postgres) ListFollowing(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error) {
	const listQ = `SELECT followee_id::text FROM follows
		WHERE follower_id = $1 ORDER BY created_at, followee_id LIMIT $2 OFFSET $3`
	const countQ = `SELECT count(*) FROM follows WHERE follower_id = $1`
	return p.pageIDs(ctx, listQ, countQ, userID, limit, offset)
}

// pageIDs ejecuta la página y su conteo total. La lista nunca es nil,
// para no sorprender al consumidor.
func (p *Postgres) pageIDs(ctx context.Context, listQ, countQ, userID string, limit, offset int32) ([]string, int32, error) {
	rows, err := p.pool.Query(ctx, listQ, userID, limit, offset)
	if err != nil {
		return nil, 0, translate(err)
	}
	ids, err := scanIDs(rows)
	rows.Close()
	if err != nil {
		return nil, 0, err
	}

	var total int32
	if err := p.pool.QueryRow(ctx, countQ, userID).Scan(&total); err != nil {
		return nil, 0, translate(err)
	}
	return ids, total, nil
}

// scanIDs recoge una única columna de texto en una lista. No cierra las
// filas: liberarlas es responsabilidad del caller.
func scanIDs(rows pgx.Rows) ([]string, error) {
	ids := make([]string, 0)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, translate(err)
		}
		ids = append(ids, id)
	}
	return ids, translate(rows.Err())
}

// Ping verifica la conexión (usado por el health check periódico).
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// Close libera el pool.
func (p *Postgres) Close() { p.pool.Close() }

// translate convierte errores del driver en errores de dominio de la
// capa store, estables para consumir desde el servicio.
func translate(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "22P02": // invalid_text_representation (uuid malformado)
			return ErrInvalidID
		case "23514": // check_violation (p. ej. follow a sí mismo)
			return ErrInvalidID
		}
	}
	return err // error de red o desconocido: se propaga tal cual
}

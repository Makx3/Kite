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

// migrationsFS empaqueta los scripts SQL de esquema para que el
// binario sea autónomo: no se necesitan archivos sueltos en la
// imagen Docker ni en el host.
//
//go:embed migrations/*.sql
var migrationsFS embed.FS

// Postgres implementa UserStore sobre un pool de pgx.
type Postgres struct {
	pool *pgxpool.Pool
}

// NewPostgres conecta y valida el pool contra la base de datos.
// Si la BD no está disponible devuelve error; el reintento al
// arrancar es responsabilidad del caller (ver cmd/server).
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

// Migrate aplica los scripts embebidos en orden lexicográfico.
// Son idempotentes (IF NOT EXISTS), por lo que ejecutarlos en cada
// arranque es seguro.
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

// Create inserta y retorna la fila completa.
func (p *Postgres) Create(ctx context.Context, username, email string) (UserRecord, error) {
	const q = `
		INSERT INTO users (username, email)
		VALUES ($1, $2)
		RETURNING user_id::text, username, email, created_at, updated_at`

	var rec UserRecord
	err := p.pool.QueryRow(ctx, q, username, email).Scan(
		&rec.ID, &rec.Username, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return UserRecord{}, translate(err)
	}
	return rec, nil
}

// Get busca por user_id; si el id ni siquiera es un UUID válido la
// BD responde error de sintaxis, que también se traduce a
// NotFound (un id malformado no puede pertenecer a nadie).
func (p *Postgres) Get(ctx context.Context, id string) (UserRecord, error) {
	const q = `
		SELECT user_id::text, username, email, created_at, updated_at
		FROM users
		WHERE user_id = $1`

	var rec UserRecord
	err := p.pool.QueryRow(ctx, q, id).Scan(
		&rec.ID, &rec.Username, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return UserRecord{}, translate(err)
	}
	return rec, nil
}

// Update aplica solo los campos provistos (COALESCE conserva el
// valor actual) y marca updated_at.
func (p *Postgres) Update(ctx context.Context, id string, username, email *string) (UserRecord, error) {
	const q = `
		UPDATE users
		SET username   = COALESCE($2, username),
		    email      = COALESCE($3, email),
		    updated_at = now()
		WHERE user_id = $1
		RETURNING user_id::text, username, email, created_at, updated_at`

	var rec UserRecord
	err := p.pool.QueryRow(ctx, q, id, username, email).Scan(
		&rec.ID, &rec.Username, &rec.Email, &rec.CreatedAt, &rec.UpdatedAt,
	)
	if err != nil {
		return UserRecord{}, translate(err)
	}
	return rec, nil
}

// Ping verifica la conexión (usado por el health check periódico).
func (p *Postgres) Ping(ctx context.Context) error { return p.pool.Ping(ctx) }

// Close libera el pool.
func (p *Postgres) Close() { p.pool.Close() }

// translate convierte errores del driver en errores de dominio de
// la capa store, estables para consumir desde el servicio.
func translate(err error) error {
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		return ErrNotFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation
			return ErrEmailTaken
		case "22P02": // invalid_text_representation (uuid malformado)
			return ErrNotFound
		}
	}
	return err // error de red o desconocido: se propaga tal cual
}

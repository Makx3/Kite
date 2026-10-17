-- Esquema inicial de users_db.
-- Cada columna refleja un dato de la entidad User; el UUID y los
-- timestamps los genera la base de datos para que ningún servicio
-- ni cliente pueda fabricar identificadores inconsistentes.

CREATE TABLE IF NOT EXISTS users (
    user_id    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    username   TEXT NOT NULL,
    email      TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Restricción de unicidad del email; el store traduce su violación
-- a ErrEmailTaken (gRPC AlreadyExists).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_email_key;
ALTER TABLE users ADD CONSTRAINT users_email_key UNIQUE (email);

-- Búsqueda por email (login futuro / deduplicación).
CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);

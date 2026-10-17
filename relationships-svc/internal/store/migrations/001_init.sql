-- Esquema inicial de relationships_db.
-- La clave primaria compuesta impide seguir dos veces a la misma
-- persona; CHECK evita que un usuario se siga a sí mismo incluso si
-- un cliente malicioso saltara la validación del servicio.

CREATE TABLE IF NOT EXISTS follows (
    follower_id UUID NOT NULL,
    followee_id UUID NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (follower_id, followee_id)
);

-- Defensa en profundidad: la regla de negocio también vive en la BD.
ALTER TABLE follows DROP CONSTRAINT IF EXISTS follows_no_self;
ALTER TABLE follows ADD CONSTRAINT follows_no_self CHECK (follower_id <> followee_id);

-- Las consultas de "¿quién sigue a X?" filtran por followee_id y las de
-- "¿a quién sigue X?" por follower_id (esta última la cubre la PK).
CREATE INDEX IF NOT EXISTS idx_follows_followee ON follows (followee_id);

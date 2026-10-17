-- Alineación con el diseño: la entidad User exige display_name
-- (VARCHAR(80) NOT NULL) y username (VARCHAR(50) UNIQUE). Va en una
-- migración aparte y no dentro de 001 porque 001 ya está aplicada:
-- una migración desplegada no se reescribe, se corrige hacia adelante.

-- Primero sin NOT NULL: una tabla con filas no acepta una columna
-- obligatoria que no tenga valor por defecto.
ALTER TABLE users ADD COLUMN IF NOT EXISTS display_name TEXT;

-- Relleno de filas preexistentes (si las hubiera) antes de exigir
-- NOT NULL; el username es el mejor valor de partida disponible.
UPDATE users SET display_name = username WHERE display_name IS NULL;

ALTER TABLE users ALTER COLUMN display_name SET NOT NULL;

-- El diseño exige unicidad de username (el email ya la tiene desde 001).
ALTER TABLE users DROP CONSTRAINT IF EXISTS users_username_key;
ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

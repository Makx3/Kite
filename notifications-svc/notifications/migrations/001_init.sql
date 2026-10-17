-- Esquema inicial de notifications_db.
--
-- Los identificadores (user_id, source_user_id, post_id) son
-- referencias lógicas a datos de otros servicios: la base NO declara
-- claves foráneas cruzadas, porque nunca podría verificarlas.
--
-- type se restringe a los dos disparadores reales del sistema y
-- post_id se exige únicamente en las notificaciones de tipo LIKE.

CREATE TABLE IF NOT EXISTS notifications (
    id             UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id        UUID NOT NULL,
    type           VARCHAR(32) NOT NULL,
    source_user_id UUID NOT NULL,
    post_id        TEXT,
    is_read        BOOLEAN NOT NULL DEFAULT false,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Defensa en profundidad del dominio: la forma de la notificación
-- también vive en la base de datos.
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_type_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_type_check
    CHECK (type IN ('LIKE', 'NEW_FOLLOWER'));

ALTER TABLE notifications DROP CONSTRAINT IF EXISTS notifications_post_id_check;
ALTER TABLE notifications ADD CONSTRAINT notifications_post_id_check
    CHECK (
        (type = 'LIKE' AND post_id IS NOT NULL)
        OR (type = 'NEW_FOLLOWER' AND post_id IS NULL)
    );

-- Índice que pide el diseño para GetUnreadNotifications: filtra por
-- destinatario y estado de lectura, y ordena por fecha descendente.
CREATE INDEX IF NOT EXISTS idx_notifications_unread
    ON notifications (user_id, is_read, created_at DESC);

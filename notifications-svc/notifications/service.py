"""Lógica de negocio del servicio de Notifications.

Valida entradas, aplica las reglas del dominio y orquesta el store;
no conoce gRPC ni PostgreSQL.
"""

from __future__ import annotations

from .domain import Notification, NotificationType
from .store import NotificationStore

# Límites de paginación de la bandeja.
DEFAULT_LIMIT = 20
MAX_LIMIT = 100


class ValidationError(Exception):
    """Entrada inválida del caller (se traduce a INVALID_ARGUMENT)."""


class NotificationService:
    """Reglas de negocio sobre las alertas de un usuario."""

    def __init__(self, store: NotificationStore) -> None:
        self._store = store

    # ── Disparadores ─────────────────────────────────────────

    def notify_new_follower(self, follower_id: str, followee_id: str) -> Notification:
        """Registra que ``follower_id`` empezó a seguir a ``followee_id``.

        El destinatario de la alerta es el seguido y el origen es el
        seguidor. Un usuario no puede seguirse a sí mismo: si llega esa
        combinación se rechaza (el servicio de Relationships ya la
        impide, esto es defensa en profundidad).
        """
        follower_id = _require("follower_id", follower_id)
        followee_id = _require("followee_id", followee_id)
        if follower_id == followee_id:
            raise ValidationError("un usuario no puede seguirse a sí mismo")

        return self._store.create(
            user_id=followee_id,
            notification_type=NotificationType.NEW_FOLLOWER,
            source_user_id=follower_id,
            post_id=None,
        )

    def notify_post_liked(
        self, post_id: str, liker_id: str, post_author_id: str
    ) -> Notification | None:
        """Registra que ``liker_id`` reaccionó a una publicación.

        Devuelve ``None`` cuando el autor reacciona a su propia
        publicación: el diseño indica que en ese caso no se genera
        notificación.
        """
        post_id = _require("post_id", post_id)
        liker_id = _require("liker_id", liker_id)
        post_author_id = _require("post_author_id", post_author_id)
        if liker_id == post_author_id:
            return None

        return self._store.create(
            user_id=post_author_id,
            notification_type=NotificationType.LIKE,
            source_user_id=liker_id,
            post_id=post_id,
        )

    # ── Bandeja ──────────────────────────────────────────────

    def get_unread_notifications(
        self, user_id: str, limit: int, offset: int
    ) -> tuple[list[Notification], int]:
        """Devuelve ``(notificaciones sin leer, total)`` del usuario."""
        user_id = _require("user_id", user_id)

        if limit <= 0:
            limit = DEFAULT_LIMIT
        limit = min(limit, MAX_LIMIT)
        offset = max(offset, 0)

        return self._store.list_unread(user_id, limit, offset)

    def mark_as_read(self, user_id: str, notification_id: str) -> Notification:
        """Marca como leída una notificación del usuario indicado."""
        user_id = _require("user_id", user_id)
        notification_id = _require("notification_id", notification_id)
        return self._store.mark_as_read(user_id, notification_id)


def _require(name: str, value: str) -> str:
    """Normaliza un campo obligatorio y falla si viene vacío."""
    value = (value or "").strip()
    if not value:
        raise ValidationError(f"{name} es obligatorio")
    return value

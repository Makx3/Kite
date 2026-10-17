"""Abstracción de persistencia del servicio de Notifications.

La capa de servicio depende solo de esta interfaz: los tests usan un
doble en memoria y PostgreSQL se usa en producción, sin que el
negocio cambie.
"""

from __future__ import annotations

from abc import ABC, abstractmethod

from .domain import Notification, NotificationType


class StoreError(Exception):
    """Error genérico de la capa de persistencia."""


class NotFoundError(StoreError):
    """La notificación no existe (o no pertenece al usuario indicado)."""


class InvalidIDError(StoreError):
    """Un identificador no tiene formato de UUID válido."""


class NotificationStore(ABC):
    """Interfaz de persistencia de notificaciones."""

    @abstractmethod
    def create(
        self,
        user_id: str,
        notification_type: NotificationType,
        source_user_id: str,
        post_id: str | None,
    ) -> Notification:
        """Persiste una notificación y la devuelve con su id y fecha."""

    @abstractmethod
    def list_unread(
        self, user_id: str, limit: int, offset: int
    ) -> tuple[list[Notification], int]:
        """Devuelve ``(notificaciones sin leer, total)`` del usuario."""

    @abstractmethod
    def mark_as_read(self, user_id: str, notification_id: str) -> Notification:
        """Marca una notificación como leída.

        Lanza :class:`NotFoundError` si no existe o no pertenece a
        ``user_id`` (un usuario no puede leer alertas ajenas).
        """

    @abstractmethod
    def ping(self) -> None:
        """Verifica la conectividad con la base de datos."""

    @abstractmethod
    def close(self) -> None:
        """Libera los recursos del pool de conexiones."""

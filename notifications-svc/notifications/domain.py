"""Entidades de negocio del servicio de Notifications.

Este paquete no depende de gRPC, ni de psycopg, ni del transporte:
modela únicamente el dominio.
"""

from __future__ import annotations

from dataclasses import dataclass
from datetime import datetime
from enum import Enum


class NotificationType(str, Enum):
    """Los dos disparadores reales del sistema.

    Hereda de ``str`` para que ``NotificationType.LIKE == "LIKE"``,
    lo que simplifica la (de)serialización contra la base de datos,
    que guarda el nombre del tipo en una columna VARCHAR.
    """

    NEW_FOLLOWER = "NEW_FOLLOWER"
    LIKE = "LIKE"


@dataclass(frozen=True)
class Notification:
    """Alerta dirigida a ``user_id`` por una acción de ``source_user_id``.

    ``post_id`` solo tiene valor en notificaciones de tipo ``LIKE``.
    Los identificadores son referencias lógicas a los servicios Users
    y Posts: no son claves foráneas reales.
    """

    notification_id: str
    type: NotificationType
    user_id: str
    source_user_id: str
    post_id: str | None
    is_read: bool
    created_at: datetime

"""Servicer gRPC de Notifications.

Traduce mensajes protobuf a llamadas del servicio de negocio. No
contiene reglas de negocio ni manejo de errores: los errores del
dominio se propagan y los traduce la capa de interceptores
(:mod:`notifications.interceptors`), que es la única que conoce gRPC.
"""

from __future__ import annotations

from pb import notifications_pb2 as pb2
from pb import notifications_pb2_grpc as pb2_grpc

from .domain import Notification, NotificationType
from .service import NotificationService

# Correspondencia entre el enum de dominio y el enum del contrato.
_TO_PROTO_TYPE = {
    NotificationType.NEW_FOLLOWER: pb2.NOTIFICATION_TYPE_NEW_FOLLOWER,
    NotificationType.LIKE: pb2.NOTIFICATION_TYPE_LIKE,
}


class NotificationServiceServicer(pb2_grpc.NotificationServiceServicer):
    """Implementación de ``notifications.NotificationService``."""

    def __init__(self, service: NotificationService) -> None:
        self._service = service

    # ── Disparadores ─────────────────────────────────────────

    def NotifyNewFollower(self, request, context):  # noqa: N802 - nombre del RPC
        notification = self._service.notify_new_follower(
            request.follower_id, request.followee_id
        )
        return pb2.NotifyNewFollowerResponse(notification=_to_proto(notification))

    def NotifyPostLiked(self, request, context):  # noqa: N802
        notification = self._service.notify_post_liked(
            request.post_id, request.liker_id, request.post_author_id
        )
        # Sin notificación (el autor reaccionó a su propia publicación):
        # se responde sin el campo notification, indicando que no se creó.
        if notification is None:
            return pb2.NotifyPostLikedResponse()
        return pb2.NotifyPostLikedResponse(notification=_to_proto(notification))

    # ── Bandeja ──────────────────────────────────────────────

    def GetUnreadNotifications(self, request, context):  # noqa: N802
        notifications, total = self._service.get_unread_notifications(
            request.user_id, request.limit, request.offset
        )
        return pb2.GetUnreadNotificationsResponse(
            notifications=[_to_proto(n) for n in notifications],
            total=total,
        )

    def MarkAsRead(self, request, context):  # noqa: N802
        notification = self._service.mark_as_read(
            request.user_id, request.notification_id
        )
        return pb2.MarkAsReadResponse(notification=_to_proto(notification))


def _to_proto(notification: Notification) -> pb2.Notification:
    message = pb2.Notification(
        notification_id=notification.notification_id,
        type=_TO_PROTO_TYPE[notification.type],
        user_id=notification.user_id,
        source_user_id=notification.source_user_id,
        is_read=notification.is_read,
    )
    if notification.post_id is not None:
        message.post_id = notification.post_id
    if notification.created_at is not None:
        message.created_at.FromDatetime(notification.created_at)
    return message

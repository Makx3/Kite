"""Pruebas unitarias de la lógica de negocio de Notifications.

Usan un doble de :class:`NotificationStore` en memoria para aislar las
reglas de negocio de PostgreSQL.
"""

from __future__ import annotations

import dataclasses
from datetime import datetime, timezone

import pytest

from notifications.domain import Notification, NotificationType
from notifications.service import (
    DEFAULT_LIMIT,
    MAX_LIMIT,
    NotificationService,
    ValidationError,
)
from notifications.store import NotFoundError


class FakeStore:
    """Doble en memoria de NotificationStore."""

    def __init__(self) -> None:
        self._items: dict[str, Notification] = {}
        self._seq = 0
        self.closed = False

    def create(self, user_id, notification_type, source_user_id, post_id):
        self._seq += 1
        notification = Notification(
            notification_id=f"notif-{self._seq}",
            type=notification_type,
            user_id=user_id,
            source_user_id=source_user_id,
            post_id=post_id,
            is_read=False,
            created_at=datetime.now(timezone.utc),
        )
        self._items[notification.notification_id] = notification
        return notification

    def list_unread(self, user_id, limit, offset):
        unread = [
            n
            for n in self._items.values()
            if n.user_id == user_id and not n.is_read
        ]
        unread.sort(key=lambda n: n.created_at, reverse=True)
        return unread[offset : offset + limit], len(unread)

    def mark_as_read(self, user_id, notification_id):
        found = self._items.get(notification_id)
        if found is None or found.user_id != user_id:
            raise NotFoundError("notificación no encontrada")
        updated = dataclasses.replace(found, is_read=True)
        self._items[notification_id] = updated
        return updated

    def ping(self):  # pragma: no cover - trivial
        return None

    def close(self):  # pragma: no cover - trivial
        self.closed = True


@pytest.fixture()
def service():
    return NotificationService(FakeStore())


# ── NotifyNewFollower ────────────────────────────────────────


def test_notify_new_follower_creates_for_followee(service):
    notification = service.notify_new_follower(follower_id="u-1", followee_id="u-2")

    assert notification.type is NotificationType.NEW_FOLLOWER
    assert notification.user_id == "u-2"       # destinatario: el seguido
    assert notification.source_user_id == "u-1"  # origen: el seguidor
    assert notification.post_id is None
    assert notification.is_read is False
    assert notification.created_at is not None


def test_notify_new_follower_rejects_self_follow(service):
    with pytest.raises(ValidationError):
        service.notify_new_follower(follower_id="u-1", followee_id="u-1")


@pytest.mark.parametrize(
    "follower, followee",
    [("", "u-2"), ("u-1", ""), ("   ", "u-2"), ("u-1", "  ")],
)
def test_notify_new_follower_requires_ids(service, follower, followee):
    with pytest.raises(ValidationError):
        service.notify_new_follower(follower_id=follower, followee_id=followee)


# ── NotifyPostLiked ──────────────────────────────────────────


def test_notify_post_liked_creates_for_author(service):
    notification = service.notify_post_liked(
        post_id="post-1", liker_id="u-1", post_author_id="u-2"
    )

    assert notification is not None
    assert notification.type is NotificationType.LIKE
    assert notification.user_id == "u-2"
    assert notification.source_user_id == "u-1"
    assert notification.post_id == "post-1"


def test_notify_post_liked_skips_self_like(service):
    notification = service.notify_post_liked(
        post_id="post-1", liker_id="u-1", post_author_id="u-1"
    )

    assert notification is None  # el diseño indica no generar alerta


@pytest.mark.parametrize(
    "post_id, liker, author",
    [("", "u-1", "u-2"), ("post-1", "", "u-2"), ("post-1", "u-1", "")],
)
def test_notify_post_liked_requires_ids(service, post_id, liker, author):
    with pytest.raises(ValidationError):
        service.notify_post_liked(
            post_id=post_id, liker_id=liker, post_author_id=author
        )


# ── GetUnreadNotifications ───────────────────────────────────


def test_get_unread_returns_only_unread_of_user(service):
    service.notify_new_follower("u-1", "u-2")
    service.notify_post_liked("post-1", "u-3", "u-2")
    service.notify_new_follower("u-9", "u-8")  # otro destinatario

    notifications, total = service.get_unread_notifications("u-2", limit=10, offset=0)

    assert total == 2
    assert {n.source_user_id for n in notifications} == {"u-1", "u-3"}


def test_get_unread_paginates_and_reports_total(service):
    for i in range(5):
        service.notify_new_follower(f"src-{i}", "u-2")

    notifications, total = service.get_unread_notifications("u-2", limit=2, offset=0)

    assert total == 5          # el total ignora la paginación
    assert len(notifications) == 2


def test_get_unread_applies_default_limit_when_zero(service):
    for i in range(DEFAULT_LIMIT + 5):
        service.notify_new_follower(f"src-{i}", "u-2")

    notifications, total = service.get_unread_notifications("u-2", limit=0, offset=0)

    assert total == DEFAULT_LIMIT + 5
    assert len(notifications) == DEFAULT_LIMIT


def test_get_unread_caps_limit(service):
    for i in range(MAX_LIMIT + 10):
        service.notify_new_follower(f"src-{i}", "u-2")

    notifications, _ = service.get_unread_notifications("u-2", limit=9999, offset=0)

    assert len(notifications) == MAX_LIMIT


def test_get_unread_requires_user_id(service):
    with pytest.raises(ValidationError):
        service.get_unread_notifications("  ", limit=10, offset=0)


def test_get_unread_negative_offset_is_clamped(service):
    service.notify_new_follower("u-1", "u-2")

    notifications, total = service.get_unread_notifications("u-2", limit=10, offset=-5)

    assert total == 1
    assert len(notifications) == 1


# ── MarkAsRead ───────────────────────────────────────────────


def test_mark_as_read_marks_the_notification(service):
    created = service.notify_new_follower("u-1", "u-2")

    updated = service.mark_as_read("u-2", created.notification_id)

    assert updated.is_read is True
    # Ya no aparece como pendiente.
    _, total = service.get_unread_notifications("u-2", limit=10, offset=0)
    assert total == 0


def test_mark_as_read_unknown_notification_raises(service):
    with pytest.raises(NotFoundError):
        service.mark_as_read("u-2", "no-existe")


def test_mark_as_read_of_another_user_raises(service):
    created = service.notify_new_follower("u-1", "u-2")

    with pytest.raises(NotFoundError):
        service.mark_as_read("otro-usuario", created.notification_id)


@pytest.mark.parametrize(
    "user_id, notification_id",
    [("", "n-1"), ("u-2", ""), ("  ", "n-1")],
)
def test_mark_as_read_requires_ids(service, user_id, notification_id):
    with pytest.raises(ValidationError):
        service.mark_as_read(user_id, notification_id)

"""Pruebas de integración del store contra PostgreSQL real.

Se omiten si ``NOTIFICATIONS_TEST_DB_DSN`` no está definida, de modo
que ``pytest`` siga siendo ejecutable sin infraestructura.

Para correrlas:

    docker run -d --name kite-pg-notif -e POSTGRES_USER=kite \\
      -e POSTGRES_PASSWORD=kite123 -e POSTGRES_DB=notifications_test \\
      -p 55434:5432 postgres:16

    NOTIFICATIONS_TEST_DB_DSN="postgres://kite:kite123@localhost:55434/notifications_test?sslmode=disable" \\
      pytest tests/test_postgres_store.py -v
"""

from __future__ import annotations

import os
import uuid

import pytest

from notifications.domain import NotificationType
from notifications.postgres_store import PostgresNotificationStore
from notifications.store import InvalidIDError, NotFoundError

DSN = os.environ.get("NOTIFICATIONS_TEST_DB_DSN")

pytestmark = pytest.mark.skipif(
    not DSN, reason="NOTIFICATIONS_TEST_DB_DSN no definida: se omite la integración"
)


@pytest.fixture(scope="module")
def store():
    st = PostgresNotificationStore(dsn=DSN, max_conns=5, statement_timeout_ms=5000)
    st.migrate()
    yield st
    st.close()


@pytest.fixture()
def ids():
    """UUIDs válidos y únicos por prueba para no chocar entre corridas."""
    return {
        "user": str(uuid.uuid4()),
        "actor": str(uuid.uuid4()),
        "author": str(uuid.uuid4()),
        "post": uuid.uuid4().hex,  # ObjectId simulado: cadena hex
    }


def test_create_new_follower(store, ids):
    notification = store.create(
        user_id=ids["user"],
        notification_type=NotificationType.NEW_FOLLOWER,
        source_user_id=ids["actor"],
        post_id=None,
    )

    assert notification.type is NotificationType.NEW_FOLLOWER
    assert notification.user_id == ids["user"]
    assert notification.source_user_id == ids["actor"]
    assert notification.post_id is None
    assert notification.is_read is False
    assert notification.created_at is not None


def test_create_like_with_post_id(store, ids):
    notification = store.create(
        user_id=ids["author"],
        notification_type=NotificationType.LIKE,
        source_user_id=ids["actor"],
        post_id=ids["post"],
    )

    assert notification.type is NotificationType.LIKE
    assert notification.post_id == ids["post"]


def test_like_without_post_id_is_rejected_by_db(store, ids):
    # La restricción CHECK exige post_id en las notificaciones LIKE.
    with pytest.raises(Exception):
        store.create(
            user_id=ids["author"],
            notification_type=NotificationType.LIKE,
            source_user_id=ids["actor"],
            post_id=None,
        )


def test_malformed_uuid_reports_invalid_id(store, ids):
    with pytest.raises(InvalidIDError):
        store.create(
            user_id="no-es-uuid",
            notification_type=NotificationType.NEW_FOLLOWER,
            source_user_id=ids["actor"],
            post_id=None,
        )


def test_list_unread_and_mark_as_read(store, ids):
    first = store.create(
        ids["user"], NotificationType.NEW_FOLLOWER, ids["actor"], None
    )
    second = store.create(
        ids["user"], NotificationType.NEW_FOLLOWER, ids["author"], None
    )

    unread, total = store.list_unread(ids["user"], limit=10, offset=0)
    assert total == 2
    # Orden descendente por fecha: la segunda es la más reciente.
    assert [n.notification_id for n in unread] == [second.notification_id, first.notification_id]

    updated = store.mark_as_read(ids["user"], first.notification_id)
    assert updated.is_read is True

    _, total_after = store.list_unread(ids["user"], limit=10, offset=0)
    assert total_after == 1


def test_mark_as_read_unknown_raises(store, ids):
    with pytest.raises(NotFoundError):
        store.mark_as_read(ids["user"], str(uuid.uuid4()))


def test_mark_as_read_of_another_user_raises(store, ids):
    notification = store.create(
        ids["user"], NotificationType.NEW_FOLLOWER, ids["actor"], None
    )
    with pytest.raises(NotFoundError):
        store.mark_as_read(ids["author"], notification.notification_id)

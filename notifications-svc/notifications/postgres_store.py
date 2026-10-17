"""Implementación PostgreSQL de :class:`NotificationStore` con psycopg 3.

El servicio usa un pool de conexiones thread-safe porque el servidor
gRPC atiende cada RPC en un hilo distinto. Cada conexión lleva un
``statement_timeout`` para que una consulta colgada no bloquee la
petición de forma indefinida.
"""

from __future__ import annotations

import logging
from pathlib import Path

import psycopg
from psycopg import errors as pg_errors
from psycopg_pool import ConnectionPool

from .domain import Notification, NotificationType
from .store import InvalidIDError, NotificationStore, NotFoundError, StoreError

logger = logging.getLogger(__name__)

# Directorio con los scripts SQL; se aplican en orden lexicográfico.
MIGRATIONS_DIR = Path(__file__).resolve().parent / "migrations"

# Columnas que devuelve cada consulta, en el orden que espera
# _row_to_notification. Se castean los UUID a texto para exponerlos
# como string en el contrato.
_SELECT_COLUMNS = (
    "id::text, user_id::text, type, source_user_id::text, post_id, is_read, created_at"
)


class PostgresNotificationStore(NotificationStore):
    """Persistencia de notificaciones sobre PostgreSQL."""

    def __init__(
        self,
        dsn: str,
        max_conns: int,
        statement_timeout_ms: int,
        pool_timeout: float = 5.0,
    ) -> None:
        def _configure(conn: psycopg.Connection) -> None:
            # Cota superior de tiempo por sentencia, aplicada a toda
            # conexión nueva del pool. Hay que commitear: el pool
            # descarta las conexiones que quedan dentro de una
            # transacción abierta.
            conn.execute(f"SET statement_timeout = {int(statement_timeout_ms)}")
            conn.commit()

        self._pool_timeout = pool_timeout
        self._pool = ConnectionPool(
            conninfo=dsn,
            min_size=1,
            max_size=max_conns,
            open=False,
            configure=_configure,
            name="notifications-db",
            # Tiempo máximo de espera por una conexión libre. Sin esta
            # cota, una base caída bloquearía cada RPC hasta 30 s.
            timeout=pool_timeout,
        )
        # wait=True propaga el fallo si la base no responde al arrancar,
        # de modo que el caller pueda reintentar.
        self._pool.open(wait=True, timeout=5.0)

    # ── Migraciones ──────────────────────────────────────────

    def migrate(self) -> None:
        """Aplica los scripts SQL embebidos en el paquete.

        Son idempotentes (IF NOT EXISTS / DROP+ADD CONSTRAINT), por lo
        que ejecutarlos en cada arranque es seguro.
        """
        if not MIGRATIONS_DIR.is_dir():
            raise StoreError(f"no se encontró el directorio de migraciones: {MIGRATIONS_DIR}")

        for path in sorted(MIGRATIONS_DIR.glob("*.sql")):
            sql = path.read_text(encoding="utf-8")
            with self._pool.connection() as conn:
                # Sin parámetros, psycopg usa el protocolo simple y
                # admite varias sentencias en una sola llamada.
                conn.execute(sql)
            logger.info("migración aplicada", extra={"archivo": path.name})

    # ── Operaciones ──────────────────────────────────────────

    def create(
        self,
        user_id: str,
        notification_type: NotificationType,
        source_user_id: str,
        post_id: str | None,
    ) -> Notification:
        sql = f"""
            INSERT INTO notifications (user_id, type, source_user_id, post_id)
            VALUES (%s, %s, %s, %s)
            RETURNING {_SELECT_COLUMNS}
        """
        try:
            with self._pool.connection() as conn:
                row = conn.execute(
                    sql, (user_id, notification_type.value, source_user_id, post_id)
                ).fetchone()
        except Exception as exc:  # noqa: BLE001 - se traduce y se relanza
            raise _translate(exc) from exc
        return _row_to_notification(row)

    def list_unread(
        self, user_id: str, limit: int, offset: int
    ) -> tuple[list[Notification], int]:
        try:
            with self._pool.connection() as conn:
                rows = conn.execute(
                    f"""
                    SELECT {_SELECT_COLUMNS}
                    FROM notifications
                    WHERE user_id = %s AND is_read = false
                    ORDER BY created_at DESC
                    LIMIT %s OFFSET %s
                    """,
                    (user_id, limit, offset),
                ).fetchall()
                total = conn.execute(
                    "SELECT count(*) FROM notifications WHERE user_id = %s AND is_read = false",
                    (user_id,),
                ).fetchone()[0]
        except Exception as exc:  # noqa: BLE001
            raise _translate(exc) from exc
        return [_row_to_notification(row) for row in rows], int(total)

    def mark_as_read(self, user_id: str, notification_id: str) -> Notification:
        try:
            with self._pool.connection() as conn:
                row = conn.execute(
                    f"""
                    UPDATE notifications
                    SET is_read = true
                    WHERE id = %s AND user_id = %s
                    RETURNING {_SELECT_COLUMNS}
                    """,
                    (notification_id, user_id),
                ).fetchone()
        except Exception as exc:  # noqa: BLE001
            raise _translate(exc) from exc

        # Sin fila afectada: la notificación no existe o es de otro
        # usuario (no se distingue, para no filtrar existencia ajena).
        if row is None:
            raise NotFoundError("notificación no encontrada")
        return _row_to_notification(row)

    def ping(self) -> None:
        # Timeout corto: el health check debe reaccionar rápido a una
        # base caída, no esperar el timeout completo del pool.
        with self._pool.connection(timeout=min(2.0, self._pool_timeout)) as conn:
            conn.execute("SELECT 1")

    def close(self) -> None:
        self._pool.close()


def _row_to_notification(row: tuple) -> Notification:
    (notification_id, user_id, ntype, source_user_id, post_id, is_read, created_at) = row
    return Notification(
        notification_id=notification_id,
        type=NotificationType(ntype),
        user_id=user_id,
        source_user_id=source_user_id,
        post_id=post_id,
        is_read=is_read,
        created_at=created_at,
    )


def _translate(exc: Exception) -> StoreError:
    """Convierte errores del driver en errores de la capa store."""
    if isinstance(exc, StoreError):
        return exc
    if isinstance(exc, pg_errors.InvalidTextRepresentation):
        # 22P02: un identificador no tiene formato de UUID.
        return InvalidIDError("identificador con formato inválido")
    if isinstance(exc, pg_errors.CheckViolation):
        return StoreError("la notificación no cumple las restricciones del dominio")
    return StoreError(f"error de base de datos: {exc}")

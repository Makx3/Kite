"""Punto de entrada del microservicio de Notifications.

Levanta el servidor gRPC con reintentos de arranque contra PostgreSQL,
health check periódico y apagado ordenado ante SIGINT/SIGTERM.
"""

from __future__ import annotations

import logging
import os
import signal
import sys
import threading
import time
from concurrent import futures

import grpc
from grpc_health.v1 import health, health_pb2, health_pb2_grpc
from grpc_reflection.v1alpha import reflection

from pb import notifications_pb2 as pb2
from pb import notifications_pb2_grpc as pb2_grpc

from notifications import config, interceptors
from notifications.postgres_store import PostgresNotificationStore
from notifications.service import NotificationService
from notifications.servicer import NotificationServiceServicer

logger = logging.getLogger("notifications")

# Cantidad de hilos que atienden RPC; cada uno puede tomar una conexión
# del pool de la base de datos.
_MAX_WORKERS = 10


def _retry_backoff(attempt: int) -> float:
    """Backoff exponencial con tope, para no martillar la base."""
    return min(0.5 * (2 ** attempt), 5.0)


def _connect_with_retry(cfg: config.Config, stop_event: threading.Event):
    """Conecta y migra la base, reintentando si aún no está lista."""
    attempts = cfg.db_connect_retries
    last_error: Exception | None = None

    for attempt in range(attempts):
        if stop_event.is_set():
            raise RuntimeError("apagado solicitado durante la conexión")
        if attempt > 0:
            time.sleep(_retry_backoff(attempt))
        try:
            store = PostgresNotificationStore(
                dsn=cfg.db_dsn,
                max_conns=cfg.db_max_conns,
                statement_timeout_ms=int(cfg.rpc_timeout * 1000),
                pool_timeout=cfg.rpc_timeout,
            )
            store.migrate()
            return store
        except Exception as err:  # noqa: BLE001 - se reintenta
            last_error = err
            logger.warning(
                "base de datos no disponible, reintentando",
                extra={"intento": attempt + 1, "error": str(err)},
            )

    raise RuntimeError(
        f"sin conexión a la base de datos tras {attempts} intentos: {last_error}"
    )


def _health_loop(store, health_servicer, interval: float, stop_event: threading.Event) -> None:
    """Pinguea la base y actualiza el estado del health check gRPC."""
    while not stop_event.wait(interval):
        try:
            store.ping()
            status = health_pb2.HealthCheckResponse.SERVING
        except Exception as err:  # noqa: BLE001 - se reporta como no saludable
            logger.warning(
                "health check: base de datos no responde", extra={"error": str(err)}
            )
            status = health_pb2.HealthCheckResponse.NOT_SERVING
        health_servicer.set("", status)


def run() -> int:
    """Arranca el servicio; devuelve el código de salida del proceso."""
    try:
        cfg = config.from_env()
    except config.ConfigError as err:
        logger.error("%s", err)
        return 1

    stop_event = threading.Event()

    def _handle_signal(signum, _frame):
        logger.info(
            "señal %s recibida: iniciando apagado",
            signal.Signals(signum).name,
        )
        stop_event.set()

    signal.signal(signal.SIGINT, _handle_signal)
    signal.signal(signal.SIGTERM, _handle_signal)

    try:
        store = _connect_with_retry(cfg, stop_event)
    except Exception as err:  # noqa: BLE001
        logger.error("%s", err)
        return 1
    logger.info("conectado a PostgreSQL y migraciones aplicadas")

    server = grpc.server(
        futures.ThreadPoolExecutor(max_workers=_MAX_WORKERS),
        interceptors=interceptors.build(logger),
    )
    pb2_grpc.add_NotificationServiceServicer_to_server(
        NotificationServiceServicer(NotificationService(store)), server
    )

    health_servicer = health.HealthServicer()
    health_servicer.set("", health_pb2.HealthCheckResponse.SERVING)
    health_pb2_grpc.add_HealthServicer_to_server(health_servicer, server)

    reflection.enable_server_reflection(
        (
            pb2.DESCRIPTOR.services_by_name["NotificationService"].full_name,
            reflection.SERVICE_NAME,
        ),
        server,
    )

    bound = server.add_insecure_port(f"0.0.0.0:{cfg.grpc_port}")
    if bound == 0:
        logger.error("no se pudo escuchar en el puerto %s", cfg.grpc_port)
        store.close()
        return 1

    server.start()
    logger.info("servidor gRPC escuchando", extra={"puerto": cfg.grpc_port})

    threading.Thread(
        target=_health_loop,
        args=(store, health_servicer, cfg.db_health_interval, stop_event),
        daemon=True,
    ).start()

    # Bloquea hasta recibir SIGINT/SIGTERM.
    stop_event.wait()
    logger.info("apagando: drenando peticiones activas")

    stopped = server.stop(grace=cfg.shutdown_timeout)
    if not stopped.wait(timeout=cfg.shutdown_timeout + 2):
        logger.warning("el drenado excedió el tiempo límite")

    store.close()
    logger.info("servicio detenido ordenadamente")
    return 0


class _ExtraFormatter(logging.Formatter):
    """Formateador que agrega los campos estructurados al mensaje.

    Los interceptores adjuntan ``metodo``, ``duracion_ms`` y ``codigo``
    a cada registro; el formato por defecto no los mostraría.
    """

    _FIELDS = ("metodo", "duracion_ms", "codigo")

    def format(self, record: logging.LogRecord) -> str:
        base = super().format(record)
        extras = " ".join(
            f"{name}={getattr(record, name)}"
            for name in self._FIELDS
            if hasattr(record, name)
        )
        return f"{base} {extras}".rstrip()


def main() -> None:
    logging.basicConfig(
        level=os.environ.get("NOTIFICATIONS_LOG_LEVEL", "INFO").upper(),
        handlers=[logging.StreamHandler()],
    )
    # El pool reintenta reconectar en segundo plano y registra cada
    # intento como WARNING; con la base caída eso inunda el log. El
    # estado real se comunica con el health check, así que se sube el
    # nivel del logger del pool para no perder las señales útiles.
    logging.getLogger("psycopg.pool").setLevel(logging.ERROR)

    # Formato común + los campos estructurados que agregan los
    # interceptores (metodo, duracion_ms, codigo).
    for handler in logging.getLogger().handlers:
        handler.setFormatter(_ExtraFormatter("%(asctime)s %(levelname)s %(name)s %(message)s"))
    sys.exit(run())


if __name__ == "__main__":
    main()

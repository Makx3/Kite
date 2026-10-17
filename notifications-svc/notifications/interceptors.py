"""Interceptores de resiliencia del servidor gRPC.

Se aplican a todas las RPC unarias del servicio, en este orden (el
primero es el más externo):

* :class:`ErrorInterceptor` traduce cualquier excepción a un estado
  gRPC ordenado. Es el único punto que llama a ``context.abort``;
  garantiza que un fallo en un handler nunca derribe el proceso.
* :class:`LoggingInterceptor` registra método, duración y código. Va
  por dentro del anterior para poder registrar el código real antes de
  que ``abort`` finalice la RPC.
* :class:`DeadlineInterceptor` rechaza una RPC cuyo plazo ya venció.

Nota sobre el tiempo límite: los hilos de Python no se pueden
interrumpir a la fuerza, por lo que el límite duro de tiempo no se
impone aquí sino en la base de datos (``statement_timeout`` por
conexión, ver :mod:`notifications.postgres_store`). El cliente, además,
impone su propio deadline.
"""

from __future__ import annotations

import logging
import time

import grpc

from . import rpc_errors


def _unary_handler(handler):
    """Devuelve el handler si es unario; ``None`` en caso contrario."""
    if handler is not None and handler.unary_unary is not None:
        return handler
    return None


def _rebuild(handler, wrapper):
    """Reconstruye el handler unario envolviendo su función."""
    return grpc.unary_unary_rpc_method_handler(
        wrapper,
        request_deserializer=handler.request_deserializer,
        response_serializer=handler.response_serializer,
    )


class ErrorInterceptor(grpc.ServerInterceptor):
    """Traduce excepciones a estados gRPC; nunca deja caer el proceso."""

    def __init__(self, logger: logging.Logger) -> None:
        self._logger = logger

    def intercept_service(self, continuation, handler_call_details):
        handler = _unary_handler(continuation(handler_call_details))
        if handler is None:
            return continuation(handler_call_details)

        inner = handler.unary_unary
        method = handler_call_details.method

        def wrapper(request, context):
            try:
                return inner(request, context)
            except Exception as err:  # noqa: BLE001 - cortafuegos del servidor
                if not rpc_errors.is_expected(err):
                    self._logger.exception("error interno en %s", method)
                # context.abort() lanza una excepción que finaliza la RPC
                # con el código indicado; al estar dentro de este except,
                # no vuelve a entrar en este mismo manejador.
                context.abort(rpc_errors.code_for(err), rpc_errors.details_for(err))

        return _rebuild(handler, wrapper)


class LoggingInterceptor(grpc.ServerInterceptor):
    """Registra método, duración y código de estado de cada RPC."""

    def __init__(self, logger: logging.Logger) -> None:
        self._logger = logger

    def intercept_service(self, continuation, handler_call_details):
        handler = _unary_handler(continuation(handler_call_details))
        if handler is None:
            return continuation(handler_call_details)

        inner = handler.unary_unary
        method = handler_call_details.method

        def wrapper(request, context):
            start = time.perf_counter()
            code = grpc.StatusCode.OK
            try:
                return inner(request, context)
            except Exception as err:  # noqa: BLE001 - solo se clasifica
                code = rpc_errors.code_for(err)
                raise
            finally:
                elapsed_ms = (time.perf_counter() - start) * 1000
                self._logger.info(
                    "rpc",
                    extra={
                        "metodo": method,
                        "duracion_ms": round(elapsed_ms, 1),
                        "codigo": code.name,
                    },
                )

        return _rebuild(handler, wrapper)


class DeadlineInterceptor(grpc.ServerInterceptor):
    """Rechaza una RPC cuyo deadline ya expiró al llegar."""

    def intercept_service(self, continuation, handler_call_details):
        handler = _unary_handler(continuation(handler_call_details))
        if handler is None:
            return continuation(handler_call_details)

        inner = handler.unary_unary

        def wrapper(request, context):
            remaining = context.time_remaining()
            if remaining is not None and remaining <= 0:
                context.abort(
                    grpc.StatusCode.DEADLINE_EXCEEDED,
                    "la operación excedió el tiempo límite",
                )
            return inner(request, context)

        return _rebuild(handler, wrapper)


def build(logger: logging.Logger) -> list[grpc.ServerInterceptor]:
    """Arma la cadena de interceptores (el primero es el más externo)."""
    return [
        ErrorInterceptor(logger),
        LoggingInterceptor(logger),
        DeadlineInterceptor(),
    ]

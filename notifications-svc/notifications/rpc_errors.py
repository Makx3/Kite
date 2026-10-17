"""Traducción de errores del dominio a códigos de estado gRPC.

Vive en un solo lugar para que tanto el interceptor que aborta la RPC
como el que la registra en el log usen exactamente la misma
clasificación.
"""

from __future__ import annotations

import grpc

from .service import ValidationError
from .store import InvalidIDError, NotFoundError

# Mensaje genérico para errores internos: el caller nunca recibe
# detalles de infraestructura.
INTERNAL_MESSAGE = "error interno del servicio"

# Errores "esperados" del dominio, con un código gRPC propio.
_EXPECTED = (ValidationError, InvalidIDError, NotFoundError)


def is_expected(err: Exception) -> bool:
    """True si el error es de dominio (esperado) y no interno."""
    return isinstance(err, _EXPECTED)


def code_for(err: Exception) -> grpc.StatusCode:
    """Código gRPC que corresponde a un error."""
    if isinstance(err, (ValidationError, InvalidIDError)):
        return grpc.StatusCode.INVALID_ARGUMENT
    if isinstance(err, NotFoundError):
        return grpc.StatusCode.NOT_FOUND
    return grpc.StatusCode.INTERNAL


def details_for(err: Exception) -> str:
    """Mensaje que se expone al cliente (seguro)."""
    return str(err) if is_expected(err) else INTERNAL_MESSAGE

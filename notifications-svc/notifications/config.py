"""Configuración del servicio de Notifications.

Toda la configuración proviene de variables de entorno: no hay puertos,
DSN ni credenciales incrustados en el código. Si falta un valor
obligatorio el proceso falla al arrancar con un mensaje claro.
"""

from __future__ import annotations

import os
from dataclasses import dataclass

# Valores por defecto de los parámetros opcionales.
_DEFAULT_RPC_TIMEOUT = 5.0
_DEFAULT_SHUTDOWN_TIMEOUT = 10.0
_DEFAULT_DB_HEALTH_INTERVAL = 10.0
_DEFAULT_DB_MAX_CONNS = 10
_DEFAULT_DB_CONNECT_RETRIES = 10


class ConfigError(Exception):
    """Error de configuración detectado al arrancar."""


@dataclass(frozen=True)
class Config:
    """Configuración efectiva, ya validada."""

    grpc_port: int
    db_dsn: str
    rpc_timeout: float
    shutdown_timeout: float
    db_health_interval: float
    db_max_conns: int
    db_connect_retries: int


def _required(name: str) -> str:
    value = os.environ.get(name, "").strip()
    if not value:
        raise ConfigError(f"config: {name} es obligatoria")
    return value


def _float_env(name: str, default: float) -> float:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        value = float(raw)
    except ValueError as exc:
        raise ConfigError(f"config: {name} inválida: {raw!r} (se espera un número de segundos)") from exc
    if value <= 0:
        raise ConfigError(f"config: {name} debe ser mayor que cero: {raw!r}")
    return value


def _int_env(name: str, default: int) -> int:
    raw = os.environ.get(name, "").strip()
    if not raw:
        return default
    try:
        value = int(raw)
    except ValueError as exc:
        raise ConfigError(f"config: {name} inválida: {raw!r} (se espera un entero)") from exc
    if value < 1:
        raise ConfigError(f"config: {name} debe ser mayor o igual a 1: {raw!r}")
    return value


def from_env() -> Config:
    """Lee y valida la configuración desde el entorno.

    ``NOTIFICATIONS_GRPC_PORT`` y ``NOTIFICATIONS_DB_DSN`` son
    obligatorias; el resto tiene valores por defecto razonables.
    """

    raw_port = _required("NOTIFICATIONS_GRPC_PORT")
    try:
        port = int(raw_port)
    except ValueError as exc:
        raise ConfigError(
            f"config: NOTIFICATIONS_GRPC_PORT inválida: {raw_port!r}"
        ) from exc
    if not 1 <= port <= 65535:
        raise ConfigError(
            f"config: NOTIFICATIONS_GRPC_PORT fuera de rango (1-65535): {port}"
        )

    return Config(
        grpc_port=port,
        db_dsn=_required("NOTIFICATIONS_DB_DSN"),
        rpc_timeout=_float_env("NOTIFICATIONS_RPC_TIMEOUT", _DEFAULT_RPC_TIMEOUT),
        shutdown_timeout=_float_env(
            "NOTIFICATIONS_SHUTDOWN_TIMEOUT", _DEFAULT_SHUTDOWN_TIMEOUT
        ),
        db_health_interval=_float_env(
            "NOTIFICATIONS_DB_HEALTH_INTERVAL", _DEFAULT_DB_HEALTH_INTERVAL
        ),
        db_max_conns=_int_env("NOTIFICATIONS_DB_MAX_CONNS", _DEFAULT_DB_MAX_CONNS),
        db_connect_retries=_int_env(
            "NOTIFICATIONS_DB_CONNECT_RETRIES", _DEFAULT_DB_CONNECT_RETRIES
        ),
    )

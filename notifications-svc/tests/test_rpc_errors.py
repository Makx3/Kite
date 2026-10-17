"""Pruebas de la traducción de errores de dominio a códigos gRPC."""

from __future__ import annotations

import grpc
import pytest

from notifications import rpc_errors
from notifications.service import ValidationError
from notifications.store import InvalidIDError, NotFoundError, StoreError


@pytest.mark.parametrize(
    "err, expected",
    [
        (ValidationError("dato inválido"), grpc.StatusCode.INVALID_ARGUMENT),
        (InvalidIDError("uuid inválido"), grpc.StatusCode.INVALID_ARGUMENT),
        (NotFoundError("no existe"), grpc.StatusCode.NOT_FOUND),
        (StoreError("la base se cayó"), grpc.StatusCode.INTERNAL),
        (RuntimeError("inesperado"), grpc.StatusCode.INTERNAL),
    ],
)
def test_code_for_maps_each_error(err, expected):
    assert rpc_errors.code_for(err) is expected


def test_expected_errors_expose_their_message():
    err = ValidationError("follower_id es obligatorio")
    assert rpc_errors.is_expected(err) is True
    assert rpc_errors.details_for(err) == "follower_id es obligatorio"


def test_internal_errors_hide_details():
    err = StoreError("password=kite123 host=10.0.0.5")
    assert rpc_errors.is_expected(err) is False
    assert rpc_errors.details_for(err) == rpc_errors.INTERNAL_MESSAGE

#!/usr/bin/env bash
# Regenera los stubs de Python del servicio de Notifications.
#
# Requiere grpcio-tools (ver requirements-dev.txt). El resultado
# (notifications_pb2.py, notifications_pb2.pyi, notifications_pb2_grpc.py)
# se versiona en el repositorio, igual que los stubs de Go.
#
# Uso:  bash scripts/gen_proto.sh
set -euo pipefail

cd "$(dirname "$0")/.."

# Permite usar un intérprete concreto (p. ej. un venv):
#   PYTHON=/ruta/venv/bin/python bash scripts/gen_proto.sh
PYTHON="${PYTHON:-python}"

"$PYTHON" -m grpc_tools.protoc \
    -I. \
    --python_out=. \
    --pyi_out=. \
    --grpc_python_out=. \
    pb/notifications.proto

echo "Stubs regenerados en pb/"

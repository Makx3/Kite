# ─ Etapa de dependencias ───────────────────────────────────
FROM python:3.14-slim AS deps

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1 \
    PIP_NO_CACHE_DIR=1

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# ─ Etapa de runtime ────────────────────────────────────────
FROM python:3.14-slim

ENV PYTHONUNBUFFERED=1 \
    PYTHONDONTWRITEBYTECODE=1

WORKDIR /app

# Copiar las dependencias ya instaladas y luego solo el código necesario.
COPY --from=deps /usr/local/lib/python3.14/site-packages /usr/local/lib/python3.14/site-packages
COPY --from=deps /usr/local/bin /usr/local/bin

COPY notifications/ ./notifications/
COPY pb/ ./pb/
COPY main.py ./

RUN useradd --create-home --uid 10001 kite
USER kite

# El puerto expuesto es solo documentación: el valor real lo define
# NOTIFICATIONS_GRPC_PORT en el entorno (compose/.env).
EXPOSE 50053

ENTRYPOINT ["python", "main.py"]

#!/usr/bin/env python3
"""Carga un dataset de ejemplo en Kite a través del API Gateway.

Solo usa la biblioteca estándar y entra por HTTP al único puerto publicado:
el gateway traduce cada llamada a gRPC, así que este script sirve también como
prueba de humo de los endpoints y del disparador de notificaciones.

    python3 scripts/seed-demo.py
    python3 scripts/seed-demo.py --base http://localhost:8080
    python3 scripts/seed-demo.py --env-file demo.env   # guarda los identificadores

Si el sistema ya tiene datos con esos usuarios, el script lo avisa y sugiere
reiniciar limpio con:  docker compose down -v && docker compose up -d
"""
import argparse
import json
import os
import sys
import urllib.error
import urllib.request

# username -> (email, display_name). El username es la clave del dataset.
USUARIOS = {
    "diego":  ("diego@ufro.cl",  "Diego Beñaldo"),
    "maxi":   ("maxi@ufro.cl",   "Maximiliano Sepúlveda"),
    "carola": ("carola@ufro.cl", "Carola Fuentes"),
    "tomas":  ("tomas@ufro.cl",  "Tomás Herrera"),
    "paula":  ("paula@ufro.cl",  "Paula Contreras"),
}

# (quién sigue, a quién sigue) — 8 aristas, sin auto-seguimiento.
SEGUIMIENTOS = [
    ("diego", "maxi"),
    ("diego", "carola"),
    ("maxi", "diego"),
    ("maxi", "tomas"),
    ("carola", "diego"),
    ("tomas", "carola"),
    ("tomas", "paula"),
    ("paula", "maxi"),
]

BASE = ""


def call(method, path, body=None, user=None):
    """Una petición al gateway; devuelve (estado, cuerpo ya decodificado)."""
    data = json.dumps(body).encode() if body is not None else None
    req = urllib.request.Request(BASE + path, data=data, method=method)
    if data is not None:
        req.add_header("Content-Type", "application/json")
    if user:
        req.add_header("X-User-Id", user)
    try:
        with urllib.request.urlopen(req) as resp:
            raw = resp.read().decode()
            return resp.status, (json.loads(raw) if raw else None)
    except urllib.error.HTTPError as err:
        raw = err.read().decode()
        try:
            return err.code, json.loads(raw)
        except json.JSONDecodeError:
            return err.code, raw
    except urllib.error.URLError as err:
        print(f"\n  No pude hablar con el gateway en {BASE}: {err.reason}")
        print("  ¿Está levantado?  docker compose up -d")
        raise SystemExit(1)


def morir(msg):
    print(f"\n  ERROR: {msg}")
    print("  Si la base ya tenía datos, reinicia limpio con:")
    print("    docker compose down -v && docker compose up -d")
    raise SystemExit(1)


def main():
    global BASE

    parser = argparse.ArgumentParser(description="Dataset de ejemplo para Kite.")
    parser.add_argument("--base", default=os.environ.get("KITE_BASE", "http://localhost:8080"),
                        help="URL del API Gateway (por defecto http://localhost:8080)")
    parser.add_argument("--env-file", default="",
                        help="archivo opcional donde dejar los identificadores creados")
    args = parser.parse_args()
    BASE = args.base.rstrip("/")

    print("=" * 68)
    print("  Dataset de ejemplo de Kite (5 usuarios, 8 seguimientos)")
    print(f"  Gateway: {BASE}")
    print("=" * 68)

    # ── 1. Usuarios ─────────────────────────────────────────────────
    print("\n1) Usuarios  —  POST /users")
    ids = {}
    for username, (email, nombre) in USUARIOS.items():
        estado, datos = call("POST", "/users",
                             {"username": username, "email": email, "display_name": nombre})
        if estado != 201:
            morir(f"crear {username} devolvió {estado}: {datos}")
        ids[username] = datos["user_id"]
        print(f"   {estado}  {username:<7} {nombre:<24} {datos['user_id']}")

    # ── 2. Grafo de seguimiento ─────────────────────────────────────
    print("\n2) Grafo de seguimiento  —  POST /users/{user_id}/follow  (X-User-Id = quién sigue)")
    for sigue, seguido in SEGUIMIENTOS:
        estado, datos = call("POST", f"/users/{ids[seguido]}/follow", user=ids[sigue])
        if estado != 201:
            morir(f"{sigue} -> {seguido} devolvió {estado}: {datos}")
        print(f"   {estado}  {sigue:<7} -> {seguido}")

    # ── 3. Recorrido por los endpoints de lectura ───────────────────
    def nombre_de(uid):
        """Hidrata un identificador con GET /users/{user_id}."""
        estado, usuario = call("GET", f"/users/{uid}")
        return f"{usuario['username']} ({usuario['display_name']})" if estado == 200 else uid

    print("\n3) Consulta  —  GET /users/{user_id}")
    estado, diego = call("GET", f"/users/{ids['diego']}")
    print(f"   -> {estado}\n" + json.dumps(diego, indent=6, ensure_ascii=False))

    print("\n4) Grafo  —  GET /users/{user_id}/followers | /following")
    for quien in ("diego", "maxi"):
        for relacion in ("followers", "following"):
            estado, pagina = call("GET", f"/users/{ids[quien]}/{relacion}?limit=10")
            detalle = ", ".join(nombre_de(i) for i in pagina["user_ids"]) or "(ninguno)"
            print(f"   {quien:<6} {relacion:<10} total={pagina['total']:<2} -> {detalle}")

    print("\n5) Bandeja agregada  —  GET /notifications  (X-User-Id = dueño)")
    estado, bandeja = call("GET", "/notifications?limit=10", user=ids["maxi"])
    print(f"   bandeja de maxi -> {estado}, total={bandeja['total']}")
    for alerta in bandeja["notifications"]:
        emisor = alerta.get("source_user", {}).get("display_name", "(omitido)")
        print(f"     · {alerta['type']:<13} de {emisor}")

    print("\n6) Marcar como leída  —  POST /notifications/{id}/read")
    primera = bandeja["notifications"][0]["notification_id"]
    estado, leida = call("POST", f"/notifications/{primera}/read", user=ids["maxi"])
    print(f"   -> {estado}, is_read={leida['is_read']}")
    _, restantes = call("GET", "/notifications?limit=10", user=ids["maxi"])
    print(f"   la bandeja queda en total={restantes['total']} (solo no leídas)")

    print("\n7) Edición parcial  —  PATCH /users/{user_id}")
    estado, editado = call("PATCH", f"/users/{ids['tomas']}", {"display_name": "Tomás Herrera R."})
    print(f"   -> {estado}, display_name='{editado['display_name']}'")

    print("\n8) Dejar de seguir  —  DELETE /users/{user_id}/follow")
    estado, _ = call("DELETE", f"/users/{ids['paula']}/follow", user=ids["tomas"])
    _, pagina = call("GET", f"/users/{ids['paula']}/followers")
    print(f"   -> {estado}; paula queda con total={pagina['total']} seguidores")

    # ── 4. Cómo seguir probando ─────────────────────────────────────
    print("\n" + "=" * 68)
    print("  Listo. Para seguir probando a mano:")
    print("=" * 68)
    print(f"""
  curl -s {BASE}/users/{ids['diego']}
  curl -s "{BASE}/users/{ids['maxi']}/followers?limit=10"
  curl -s -H "X-User-Id: {ids['maxi']}" "{BASE}/notifications?limit=10"

  # En el navegador (JSON):
  {BASE}/users/{ids['diego']}
""")

    if args.env_file:
        with open(args.env_file, "w") as fh:
            fh.write("# Identificadores del dataset de ejemplo de Kite\n")
            for username, uid in ids.items():
                fh.write(f"{username.upper()}_ID={uid}\n")
        print(f"  Identificadores guardados en {args.env_file}\n")


if __name__ == "__main__":
    try:
        main()
    except KeyboardInterrupt:
        sys.exit(130)

# Kite

Red social mínima construida como sistema de microservicios. La única superficie
externa es un **API Gateway** en HTTP/REST; por dentro todo se habla por **gRPC** y
cada servicio es dueño exclusivo de su propia base de datos.

Proyecto semestral de *Sistemas Distribuidos y Escalables* — Entrega 1.
Integrantes: **Diego Beñaldo** y **Maximiliano Sepulveda**.

---

## Arquitectura

```
                        HTTP/REST
   cliente  ─────────────────────────────▶  API Gateway  (único puerto publicado)
   (curl,                                            │
    Postman,                                         │  gRPC
    frontend)                                        ▼
                                    ┌──────────────┼──────────────┐
                                    ▼              ▼              ▼
                                Users        Relationships   Notifications
                                    │              │              │
                                    ▼              ▼              ▼
                               users_db    relationships_db  notifications_db
```

| Servicio | Lenguaje | Base de datos | Responsabilidad | Puerto interno |
| :--- | :--- | :--- | :--- | :--- |
| **API Gateway** | Go | — | Traduce REST a gRPC y **agrega** respuestas de varios servicios | `8080` (publicado) |
| **Users** | Go | PostgreSQL `users_db` | Identidad: crear, consultar y editar usuarios | `50051` |
| **Relationships** | Go | PostgreSQL `relationships_db` | Grafo de seguimiento (quién sigue a quién) | `50052` |
| **Notifications** | Python | PostgreSQL `notifications_db` | Alertas: nuevo seguidor y *like* | `50053` |

> Solo el **8080** se publica al host. Los puertos 50051–50053 son internos: los
> servicios de negocio **no** son alcanzables desde fuera de la red de Docker.

Quedan por incorporar los servicios de Posts, Feed y Likes (con MongoDB y Redis),
que se sumarán al gateway cuando estén listos.

---

## Requisitos

- Docker con Compose v2 (`docker compose`, no hace falta `docker-compose`).
- Puertos libres en el host: **8080**.

Para el desarrollo sin Docker, además: Go 1.27+ y Python 3.14+ (ver *Desarrollo local*).

---

## Puesta en marcha

```bash
git clone git@github.com:Makx3/Kite.git
cd Kite
docker compose up -d --build
```

Eso construye las cuatro imágenes, levanta las tres bases, espera a que estén
sanas y arranca los servicios y el gateway. La primera vez tarda un par de
minutos; las siguientes son segundos.

### Variables de entorno

Toda la configuración va por variables de entorno: **no hay ningún puerto, URL ni
credencial en el código**. El archivo [`.env`](.env) trae valores de ejemplo listos
para usar y Compose lo lee solo:

```dotenv
POSTGRES_USER=kite
POSTGRES_PASSWORD=kite123
GATEWAY_HTTP_PORT=8080
```

Cada servicio valida además lo suyo al arrancar y **falla con un mensaje claro si
falta algo**, en vez de tomar un valor por defecto silencioso. Las que recibe cada
contenedor ya están fijadas en `docker-compose.yml`:

| Servicio | Variables |
| :--- | :--- |
| Users | `USERS_GRPC_PORT`, `USERS_DB_DSN` |
| Relationships | `RELATIONSHIPS_GRPC_PORT`, `RELATIONSHIPS_DB_DSN`, `RELATIONSHIPS_NOTIFICATIONS_ADDR` |
| Notifications | `NOTIFICATIONS_GRPC_PORT`, `NOTIFICATIONS_DB_DSN` |
| API Gateway | `GATEWAY_HTTP_PORT`, `GATEWAY_USERS_ADDR`, `GATEWAY_RELATIONSHIPS_ADDR`, `GATEWAY_NOTIFICATIONS_ADDR` |

### Comprobar que quedó arriba

```bash
docker compose ps                       # los 7 contenedores en "Up"/"healthy"
curl -s localhost:8080/health
```

```json
{"backends":{"notifications":"IDLE","relationships":"IDLE","users":"IDLE"},"status":"ok"}
```

Los backends aparecen en `IDLE` hasta la primera llamada real; `IDLE` solo
significa "todavía no se usó".

---

## Cómo probar cada endpoint del gateway

Las operaciones que actúan **en nombre de un usuario** lo reciben en el encabezado
`X-User-Id`. La primera entrega no incluye autenticación: ese encabezado es la
identidad declarada por el cliente.

### 0. Índice de la API — `GET /`

```bash
curl -s localhost:8080/
```

```json
{"service":"Kite — API Gateway","version":"entrega-1","docs":"https://github.com/Makx3/Kite#readme","note":"Las operaciones que actúan en nombre de un usuario reciben su identidad en el encabezado X-User-Id. Este gateway es la única superficie del sistema: la comunicación interna es gRPC.","endpoints":[{"method":"GET","path":"/","description":"Índice de la API (esta respuesta)"},{"method":"POST","path":"/users","description":"Crear un usuario"}, "..."]}
```

El gateway es una API, no un sitio web: no hay HTML que navegar. Aun así, la
raíz devuelve el mapa de rutas para que abrir `http://localhost:8080/` en el
navegador sea útil en lugar de un `404` seco. Es la única ruta que no delega en
un microservicio.

### 1. Crear un usuario — `POST /users`

```bash
curl -s -X POST localhost:8080/users \
  -H 'Content-Type: application/json' \
  -d '{"username":"diego","email":"diego@ufro.cl","display_name":"Diego Beñaldo"}'
```

```json
{"user_id":"2f1a…","username":"diego","email":"diego@ufro.cl","display_name":"Diego Beñaldo","created_at":"2026-10-17T15:04:05.123Z","updated_at":"2026-10-17T15:04:05.123Z"}
```

`201 Created`. Errores: `400` si los datos no son válidos (username de 3 a 50
caracteres, email con forma de email, `display_name` de 1 a 80), `409` si el
username o el email ya existen.

### 2. Consultar un perfil — `GET /users/{user_id}`

```bash
curl -s localhost:8080/users/$USER_ID
```

`200` con el usuario, o `404` si no existe.

### 3. Editar el perfil — `PATCH /users/{user_id}`

Es un parche: solo se tocan los campos enviados. El `username` es el identificador
estable y **no** se edita.

```bash
curl -s -X PATCH localhost:8080/users/$USER_ID \
  -H 'Content-Type: application/json' \
  -d '{"display_name":"Diego B."}'
```

### 4. Seguir a un usuario — `POST /users/{user_id}/follow`

El `{user_id}` de la ruta es **a quién** se sigue; **quién** sigue va en el
encabezado.

```bash
curl -s -X POST localhost:8080/users/$BOB_ID/follow -H "X-User-Id: $USER_ID"
```

```json
{"follower_id":"2f1a…","followee_id":"9c7b…","created_at":"2026-10-17T15:06:11.451Z"}
```

`201 Created`. Repetir el mismo seguimiento es idempotente y **no** vuelve a
notificar. Seguirse a uno mismo devuelve `400`.

### 5. Consultar el grafo — `GET …/followers` y `GET …/following`

Ambos aceptan `limit` y `offset` y devuelven el total sin paginar.

```bash
curl -s "localhost:8080/users/$BOB_ID/followers?limit=10&offset=0"
curl -s "localhost:8080/users/$USER_ID/following?limit=10&offset=0"
```

```json
{"user_ids":["2f1a…"],"total":1}
```

### 6. Ver las notificaciones — `GET /notifications`

Este es el endpoint **agregado**: el gateway pide la bandeja a Notifications y
luego completa el emisor de cada alerta con Users, de modo que un solo llamado
basta.

```bash
curl -s localhost:8080/notifications -H "X-User-Id: $BOB_ID"
```

```json
{
  "notifications": [
    {
      "notification_id": "5d3e…",
      "type": "NEW_FOLLOWER",
      "user_id": "9c7b…",
      "source_user_id": "2f1a…",
      "source_user": {"user_id":"2f1a…","username":"diego","display_name":"Diego Beñaldo","…":"…"},
      "is_read": false,
      "created_at": "2026-10-17T15:06:11.461Z"
    }
  ],
  "total": 1
}
```

### 7. Marcar una notificación como leída — `POST /notifications/{id}/read`

```bash
curl -s -X POST localhost:8080/notifications/$NOTIFICATION_ID/read \
  -H "X-User-Id: $BOB_ID"
```

El usuario puede ir en el encabezado o en el cuerpo (`{"user_id":"…"}`); se acepta
cualquiera de los dos.

### 8. Dejar de seguir — `DELETE /users/{user_id}/follow`

```bash
curl -s -X DELETE localhost:8080/users/$BOB_ID/follow -H "X-User-Id: $USER_ID"
```

`204 No Content`. Si la relación no existía, `404`.

### 9. Salud — `GET /health`

```bash
curl -s localhost:8080/health
```

### Flujo completo de una sola pasada

Copia y pega: crea dos usuarios, hace que uno siga al otro, muestra la alerta
agregada, la marca como leída y deshace el seguimiento.

```bash
BASE=http://localhost:8080

ALICE=$(curl -s -X POST $BASE/users -H 'Content-Type: application/json' \
  -d '{"username":"alice","email":"alice@ufro.cl","display_name":"Alicia"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["user_id"])')

BOB=$(curl -s -X POST $BASE/users -H 'Content-Type: application/json' \
  -d '{"username":"bob","email":"bob@ufro.cl","display_name":"Bernardo"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["user_id"])')

echo "alice=$ALICE  bob=$BOB"

curl -s -X POST $BASE/users/$BOB/follow -H "X-User-Id: $ALICE"; echo
curl -s "$BASE/users/$BOB/followers"; echo
curl -s $BASE/notifications -H "X-User-Id: $BOB"; echo

NOTIF=$(curl -s $BASE/notifications -H "X-User-Id: $BOB" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["notifications"][0]["notification_id"])')

curl -s -X POST $BASE/notifications/$NOTIF/read -H "X-User-Id: $BOB"; echo
curl -s -X DELETE $BASE/users/$BOB/follow -H "X-User-Id: $ALICE"; echo
```

> `python3 -c` solo se usa para extraer un campo del JSON. Sirve igual `jq -r .user_id`.

---

## Códigos de error

Todas las respuestas de error tienen la misma forma:

```json
{"error":"service_unavailable","message":"el servicio de destino no está disponible"}
```

| HTTP | `error` | Cuándo |
| :---: | :--- | :--- |
| 400 | `invalid_argument`, `invalid_body`, `invalid_query`, `missing_user` | Datos inválidos o falta el encabezado `X-User-Id` |
| 404 | `not_found` | El recurso o la ruta no existe |
| 405 | — | Método no permitido para esa ruta |
| 409 | `already_exists` | Username o email ya registrados |
| 502 | `bad_gateway` | El servicio de destino devolvió un error inesperado |
| 503 | `service_unavailable` | El servicio de destino está caído |
| 504 | `deadline_exceeded` | El servicio de destino no respondió a tiempo |

Los mensajes de validación los produce el microservicio correspondiente y el
gateway solo los propaga. Los errores internos no se filtran al cliente.

---

## Resiliencia

El sistema está pensado para que **la caída de una pieza no tumbe el resto**. Puedes
comprobarlo:

```bash
docker compose stop users-svc

curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/users/$ALICE   # 503
curl -s localhost:8080/notifications -H "X-User-Id: $BOB"              # 200: sigue
                                                                       # respondiendo,
                                                                       # sin el emisor
curl -s localhost:8080/health                                          # 200: el gateway vive

docker compose start users-svc                                         # se recupera solo
```

- El gateway usa **conexiones perezosas**: arranca aunque un servicio esté caído.
- Cada llamada gRPC tiene su propio tiempo límite, así que un servicio lento no
  arrastra la latencia del gateway.
- `GET /notifications` **degrada de forma elegante**: si Users no responde, entrega
  igual las alertas, omitiendo el campo accesorio del emisor.
- Nada de esto requiere reiniciar el gateway.

---

## Desarrollo local (sin Docker)

Cada servicio es un módulo independiente; se compila y prueba por separado.

```bash
# Go (Users, Relationships, API Gateway)
cd users-svc              # o relationships-svc / api-gateway
gofmt -l . && go build ./... && go vet ./... && go test ./...
```

Para regenerar los stubs tras editar un `.proto` (los plugins viven en `~/go/bin`):

```bash
export PATH="$PATH:$HOME/go/bin"
cd users-svc
protoc --go_out=paths=source_relative:. --go-grpc_out=paths=source_relative:. pb/users.proto
```

```bash
# Notifications (Python)
cd notifications-svc
python3 -m venv .venv && . .venv/bin/activate
pip install -r requirements-dev.txt
python -m pytest tests/
```

Las pruebas de integración de la capa de datos se saltan salvo que exista una
base de prueba, por ejemplo `USERS_TEST_DB_DSN` o `RELATIONSHIPS_TEST_DB_DSN`.

---

## Estructura del repositorio

```
.
├── docker-compose.yml          # levanta el sistema completo
├── .env                        # valores de ejemplo de las variables de entorno
├── api-gateway/                # REST → gRPC y agregación (Go)
├── users-svc/                  # identidad (Go)
├── relationships-svc/          # grafo de seguimiento (Go)
└── notifications-svc/          # alertas (Python)
```

Cada servicio sigue la misma forma: `cmd/server` para el arranque, `internal/`
con las capas (transporte, negocio, acceso a datos) y `pb/` con el contrato
Protobuf y los stubs generados.

---

## Detener y limpiar

```bash
docker compose down          # detiene y elimina los contenedores (conserva los datos)
docker compose down -v       # además borra los volúmenes: se pierden los datos
```

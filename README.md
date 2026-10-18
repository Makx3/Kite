# Kite

Red social mínima construida como sistema de microservicios. La única superficie
externa es un **API Gateway** en HTTP/REST; por dentro todo se habla por **gRPC** y
cada servicio es dueño exclusivo de su propia base de datos.

Proyecto semestral de *Sistemas Distribuidos y Escalables* · Entrega 1.
Integrantes: **Diego Beñaldo** y **Maximiliano Sepulveda**.

---

## Arquitectura

```
                         HTTP/REST
    cliente  ─────────────────────────────▶  API Gateway  (único puerto publicado)
    (curl,                                            │
     Postman,                                         │  gRPC
     frontend)                                        ▼
             ┌────────┬───────────┬──────────────┬──────────┬──────────┬──────────┐
             ▼        ▼           ▼              ▼          ▼          ▼
           Users  Relationships  Notifications  Posts     Feed      Likes
             │        │           │              │          │          │
             ▼        ▼           ▼              ▼          ▼          ▼
        users_db relationships_db notifications_db posts_db feed_db likes_db
        PostgreSQL PostgreSQL    PostgreSQL      MongoDB   Redis    MongoDB
```

Además del reparto desde el gateway, hay llamadas gRPC entre servicios:
Relationships → Notifications (nuevo seguidor), Likes → Notifications (like),
Posts → Feed (refresca el muro) y Feed → Relationships (lista de seguidores).

| Servicio | Lenguaje | Base de datos | Responsabilidad | Puerto interno |
| :--- | :--- | :--- | :--- | :--- |
| **API Gateway** | Go | — | Traduce REST a gRPC y **agrega** respuestas de varios servicios | `8080` (publicado) |
| **Users** | Go | PostgreSQL `users_db` | Identidad: crear, consultar y editar usuarios | `50051` |
| **Relationships** | Go | PostgreSQL `relationships_db` | Grafo de seguimiento (quién sigue a quién) | `50052` |
| **Notifications** | Python | PostgreSQL `notifications_db` | Alertas: nuevo seguidor y *like* | `50053` |
| **Posts** | Go | MongoDB `posts_db` | Publicaciones: crear, consultar y repartir en los muros | `50054` |
| **Feed** | Go | Redis `feed_db` | Muro por usuario, ordenado por tiempo | `50055` |
| **Likes** | Go | MongoDB `likes_db` | Reacciones sobre una publicación | `50056` |

> Solo el **8080** se publica al host. Los puertos 50051–50056 son internos: los
> servicios de negocio **no** son alcanzables desde fuera de la red de Docker.

---

## Requisitos

- Docker con Compose v2 (`docker compose`, no hace falta `docker-compose`).
- Puertos libres en el host: **8080**.
- **Python 3** en el host: solo lo usan los ejemplos de más abajo (para extraer un
  campo del JSON) y el sembrador `scripts/seed-demo.py`. Nada del sistema lo
  necesita para funcionar.

Para el desarrollo sin Docker, además: Go 1.27+ y Python 3.14+ (ver *Desarrollo local*).

---

## Puesta en marcha

```bash
git clone git@github.com:Makx3/Kite.git
cd Kite
docker compose up -d --build
```

Eso construye las siete imágenes, levanta las seis bases, espera a que estén
sanas y arranca los seis servicios y el gateway. La primera vez tarda un par de
minutos; las siguientes son segundos.

### Variables de entorno

Toda la configuración va por variables de entorno: **no hay ningún puerto, URL ni
credencial en el código**. El archivo [`.env`](.env) trae valores de ejemplo listos
para usar y Compose lo lee solo:

```dotenv
POSTGRES_USER=kite
POSTGRES_PASSWORD=kite123
POSTS_DB_USER=kite
POSTS_DB_PASS=kite123
LIKES_DB_USER=kite
LIKES_DB_PASS=kite123
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
| Posts | `POSTS_GRPC_PORT`, `POSTS_DB_DSN`, `POSTS_FEED_ADDR` |
| Feed | `FEED_GRPC_PORT`, `FEED_DB_DSN`, `FEED_RELATIONSHIPS_ADDR` |
| Likes | `LIKES_GRPC_PORT`, `LIKES_DB_DSN`, `LIKES_NOTIFICATIONS_ADDR` |
| API Gateway | `GATEWAY_HTTP_PORT` y las direcciones `GATEWAY_*_ADDR` de los seis servicios |

Redis (la base de Feed) no lleva credencial: queda en la red interna, sin puertos
publicados ni salida al exterior.

### Comprobar que quedó arriba

```bash
docker compose ps                       # los 13 contenedores en "Up"/"healthy"
curl -s localhost:8080/health
```

```json
{"backends":{"feed":"IDLE","likes":"IDLE","notifications":"IDLE","posts":"IDLE","relationships":"IDLE","users":"IDLE"},"status":"ok"}
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

### 9. Crear una publicación — `POST /posts`

```bash
curl -s -X POST localhost:8080/posts \
  -H "X-User-Id: $USER_ID" \
  -H 'Content-Type: application/json' \
  -d '{"content":"hola desde kite"}'
```

```json
{"post_id":"6ad4a8c4…","status":"created","created_at":"2026-10-18T11:08:52.525Z"}
```

`201 Created`. El autor va en el encabezado. Al persistir, el servicio Posts
reparte el post en el muro del autor y el de sus seguidores; si Feed estuviera
caído, la publicación se conserva igual (solo se advierte en el log).

### 10. Ver una publicación — `GET /posts/{post_id}`

```bash
curl -s localhost:8080/posts/$POST_ID
```

`200` con el post, o `404` si no existe.

### 11. Dar like — `POST /posts/{post_id}/like`

```bash
curl -s -X POST localhost:8080/posts/$POST_ID/like -H "X-User-Id: $USER_ID"
```

```json
{"like_id":"6ad4a8c5…","status":"ok"}
```

`201 Created`. Repetir el like es idempotente: devuelve `already_liked` con el
mismo `like_id` y **no** vuelve a notificar. Si el post no existe, `404` (y en
ese caso el servicio de Likes ni siquiera se consulta). Cuando el like es nuevo,
Likes dispara la notificación de tipo `LIKE` al autor del post; si el autor es
quien reacciona, no hay notificación.

### 12. Quitar like — `DELETE /posts/{post_id}/like`

```bash
curl -s -X DELETE localhost:8080/posts/$POST_ID/like -H "X-User-Id: $USER_ID"
```

`204 No Content`. Si la reacción no existía, `404`.

### 13. Ver el muro — `GET /feed`

Este es el segundo endpoint **agregado**: el gateway pide el muro (identificadores
y total) a Feed y luego completa cada publicación con Posts, preservando el orden
del muro.

```bash
curl -s localhost:8080/feed -H "X-User-Id: $USER_ID"
```

```json
{"posts":[{"post_id":"6ad4a8c4…","user_id":"91da…","content":"hola desde kite","created_at":"2026-10-18T11:08:52.525Z"}],"total":1,"last_updated":"2026-10-18T11:08:52Z"}
```

`total` es el total real del muro, independiente de la página pedida
(`limit`/`offset`). Un muro vacío devuelve `posts: []` (no `null`). Si un post
del muro ya no existe, se omite con una advertencia en el log sin tumbar la
respuesta.

### 14. Salud — `GET /health`

```bash
curl -s localhost:8080/health
```

### Flujo completo de una sola pasada

Copia y pega: crea dos usuarios, hace que uno siga al otro, publica, da like,
lee el muro y las alertas de los dos tipos, marca una como leída y deshace el
seguimiento.

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

POST=$(curl -s -X POST $BASE/posts -H "X-User-Id: $BOB" \
  -H 'Content-Type: application/json' -d '{"content":"hola desde kite"}' \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["post_id"])')

curl -s -X POST $BASE/posts/$POST/like -H "X-User-Id: $ALICE"; echo
curl -s $BASE/feed -H "X-User-Id: $ALICE"; echo

curl -s $BASE/notifications -H "X-User-Id: $BOB"; echo

NOTIF=$(curl -s $BASE/notifications -H "X-User-Id: $BOB" \
  | python3 -c 'import json,sys; print(json.load(sys.stdin)["notifications"][0]["notification_id"])')

curl -s -X POST $BASE/notifications/$NOTIF/read -H "X-User-Id: $BOB"; echo
curl -s -X DELETE $BASE/posts/$POST/like -H "X-User-Id: $ALICE"; echo
curl -s -X DELETE $BASE/users/$BOB/follow -H "X-User-Id: $ALICE"; echo
```

> `python3 -c` solo se usa para extraer un campo del JSON. Sirve igual `jq -r .user_id`.

### Datos de ejemplo

Si prefieres partir de un grafo ya armado en vez de crearlo a mano, el repositorio
incluye un sembrador que entra **solo por el gateway** (y por lo tanto ejercita los
endpoints y el disparador de notificaciones de verdad):

```bash
python3 scripts/seed-demo.py
```

Crea 5 usuarios (`diego`, `maxi`, `carola`, `tomas`, `paula`) y 8 seguimientos,
recorre los endpoints de lectura y termina imprimiendo `curl`s listos para copiar
con los identificadores reales. Con `--env-file demo.env` deja además las variables
`DIEGO_ID`, `MAXI_ID`, … en un archivo para `source`arlo.

Es idempotente en el sentido de que avisa si los usuarios ya existen; para volver a
empezar limpio: `docker compose down -v && docker compose up -d`.

> Sobre la URL: el camino canónico es **sin** barra final (`/users/{user_id}`). El
> gateway tolera una barra de más (`/users/{user_id}/`) y la resuelve igual, así que
> no importa cuál de las dos se use.

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

Con el servicio Posts detenido, en cambio, **todo lo que depende de publicaciones
se traslada al cliente** en 503 (`/posts`, `/feed` y los likes), porque entregar
un muro sin contenido sería un éxito engañoso; el resto del sistema sigue en 200:

```bash
docker compose stop posts-svc

curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/posts/$POST_ID  # 503
curl -s -o /dev/null -w '%{http_code}\n' localhost:8080/users/$ALICE    # 200

docker compose start posts-svc
```

- El gateway usa **conexiones perezosas**: arranca aunque un servicio esté caído.
- Cada llamada gRPC tiene su propio tiempo límite, así que un servicio lento no
  arrastra la latencia del gateway.
- `GET /notifications` **degrada de forma elegante**: si Users no responde, entrega
  igual las alertas, omitiendo el campo accesorio del emisor.
- Los disparadores también degradan: seguir a alguien o dar like persiste aunque
  Notifications esté caído, y publicar persiste aunque Feed esté caído; en cada
  caso solo se registra una advertencia.
- Nada de esto requiere reiniciar el gateway.

---

## Desarrollo local (sin Docker)

Cada servicio es un módulo independiente; se compila y prueba por separado.

```bash
# Go (Users, Relationships, Posts, Feed, Likes, API Gateway)
cd users-svc              # o relationships-svc, posts-svc, feed-svc,
                          # likes-svc, api-gateway
gofmt -l . && go build ./... && go vet ./... && go test ./...
```

Tras editar un `.proto` de Posts, Feed o Likes, el gateway mantiene una copia del
contrato en `api-gateway/pb/`; hay que regenerar los stubs en ambos lugares (el
servicio y el gateway) con el mismo `protoc` de abajo.

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
base de prueba: `USERS_TEST_DB_DSN`, `RELATIONSHIPS_TEST_DB_DSN`,
`POSTS_TEST_DB_DSN`, `LIKES_TEST_DB_DSN` o `FEED_TEST_DB_DSN`. Las de Posts y
Likes (MongoDB) llevan la etiqueta `integration`, así que se piden
explícitamente: `go test -tags=integration ./internal/store/`.

---

## Estructura del repositorio

```
.
├── docker-compose.yml          # levanta el sistema completo
├── .env                        # valores de ejemplo de las variables de entorno
├── api-gateway/                # REST → gRPC y agregación (Go)
├── users-svc/                  # identidad (Go)
├── relationships-svc/          # grafo de seguimiento (Go)
├── notifications-svc/          # alertas (Python)
├── posts-svc/                  # publicaciones (Go + MongoDB)
├── feed-svc/                   # muro por usuario (Go + Redis)
├── likes-svc/                  # reacciones (Go + MongoDB)
├── docs/
│   ├── documento-tecnico.md    # documento técnico de la entrega
│   └── arquitectura.png        # diagrama de la arquitectura
└── scripts/
    └── seed-demo.py            # dataset de ejemplo por el gateway
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

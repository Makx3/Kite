// Package httpapi expone la superficie REST del sistema. Es la única puerta
// de entrada externa: traduce HTTP a llamadas gRPC y agrega respuestas de
// varios servicios cuando el caso de uso lo requiere. No contiene reglas de
// negocio: esas viven en cada microservicio.
package httpapi

import (
	"log/slog"
	"net/http"
	"time"
)

// Deps son las dependencias del gateway, ya construidas.
type Deps struct {
	Users          UsersBackend
	Relationships  RelationshipsBackend
	Notifications  NotificationsBackend
	Posts          PostsBackend
	Feed           FeedBackend
	Likes          LikesBackend
	HandlerTimeout time.Duration
}

// Handler implementa la superficie REST del gateway.
type Handler struct {
	users         UsersBackend
	relationships RelationshipsBackend
	notifications NotificationsBackend
	posts         PostsBackend
	feed          FeedBackend
	likes         LikesBackend
	logger        *slog.Logger
}

// NewRouter arma el enrutador con las rutas del contrato del diseño y la
// cadena de middlewares: recuperación de panics (lo más externo), log de
// acceso y plazo por petición.
func NewRouter(deps Deps, logger *slog.Logger) http.Handler {
	h := &Handler{
		users:         deps.Users,
		relationships: deps.Relationships,
		notifications: deps.Notifications,
		posts:         deps.Posts,
		feed:          deps.Feed,
		likes:         deps.Likes,
		logger:        logger,
	}

	mux := http.NewServeMux()

	// Índice de la API en la raíz. El patrón "GET /{$}" coincide solo con el
	// camino exacto "/": un comodín "/" se tragaría los caminos cuyo método no
	// coincide y rompería los 405 (por ejemplo GET /users respondería 200).
	mux.HandleFunc("GET /{$}", h.index)

	// Identidad
	mux.HandleFunc("POST /users", h.createUser)
	mux.HandleFunc("GET /users/{user_id}", h.getUser)
	mux.HandleFunc("PATCH /users/{user_id}", h.updateUser)

	// Grafo de seguimiento
	mux.HandleFunc("POST /users/{user_id}/follow", h.follow)
	mux.HandleFunc("DELETE /users/{user_id}/follow", h.unfollow)
	mux.HandleFunc("GET /users/{user_id}/followers", h.followers)
	mux.HandleFunc("GET /users/{user_id}/following", h.following)

	// Publicaciones y reacciones
	mux.HandleFunc("POST /posts", h.createPost)
	mux.HandleFunc("GET /posts/{post_id}", h.getPost)
	mux.HandleFunc("POST /posts/{post_id}/like", h.likePost)
	mux.HandleFunc("DELETE /posts/{post_id}/like", h.unlikePost)

	// Muro personal (agregación Feed + Posts)
	mux.HandleFunc("GET /feed", h.getFeed)

	// Notificaciones
	mux.HandleFunc("GET /notifications", h.listNotifications)
	mux.HandleFunc("POST /notifications/{notification_id}/read", h.markNotificationRead)

	// Salud
	mux.HandleFunc("GET /health", h.health)

	return chain(routeFallbacks(mux), recovery(logger), accessLog(logger), timeout(deps.HandlerTimeout))
}

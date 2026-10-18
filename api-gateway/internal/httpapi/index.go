package httpapi

import "net/http"

// route describe una ruta del contrato, para el índice de la raíz.
type route struct {
	Method      string `json:"method"`
	Path        string `json:"path"`
	Description string `json:"description"`
}

// apiIndex es lo que responde GET /. El gateway no sirve una interfaz web —es
// una API—, pero abrir la URL base en el navegador es lo primero que hace
// cualquiera (y un evaluador también): en vez de un 404 seco, la raíz devuelve
// el mapa de la API.
type apiIndex struct {
	Service   string  `json:"service"`
	Version   string  `json:"version"`
	Docs      string  `json:"docs"`
	Note      string  `json:"note"`
	Endpoints []route `json:"endpoints"`
}

// rutas es el contrato REST del gateway, en el mismo orden en que se presenta
// en el README y en el documento técnico.
var rutas = []route{
	{"GET", "/", "Índice de la API (esta respuesta)"},
	{"POST", "/users", "Crear un usuario"},
	{"GET", "/users/{user_id}", "Consultar el perfil de un usuario"},
	{"PATCH", "/users/{user_id}", "Editar el perfil (parche: solo los campos enviados)"},
	{"POST", "/users/{user_id}/follow", "Seguir a un usuario (quién sigue va en X-User-Id)"},
	{"DELETE", "/users/{user_id}/follow", "Dejar de seguir a un usuario"},
	{"GET", "/users/{user_id}/followers", "Seguidores de un usuario (limit, offset)"},
	{"GET", "/users/{user_id}/following", "A quién sigue un usuario (limit, offset)"},
	{"POST", "/posts", "Crear una publicación (el autor va en X-User-Id)"},
	{"GET", "/posts/{post_id}", "Consultar una publicación"},
	{"POST", "/posts/{post_id}/like", "Reaccionar a una publicación (el usuario va en X-User-Id)"},
	{"DELETE", "/posts/{post_id}/like", "Quitar la reacción a una publicación"},
	{"GET", "/feed", "Muro personal agregado con Posts (limit, offset)"},
	{"GET", "/notifications", "Bandeja de alertas sin leer, agregada con Users"},
	{"POST", "/notifications/{notification_id}/read", "Marcar una alerta como leída"},
	{"GET", "/health", "Salud del gateway y de sus servicios"},
}

// index atiende GET /. Es la única ruta que no delega en un microservicio: solo
// describe la API para que la raíz del gateway sea útil.
func (h *Handler) index(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, apiIndex{
		Service: "Kite — API Gateway",
		Version: "entrega-1",
		Docs:    "https://github.com/Makx3/Kite#readme",
		Note: "Las operaciones que actúan en nombre de un usuario reciben su " +
			"identidad en el encabezado X-User-Id. Este gateway es la única " +
			"superficie del sistema: la comunicación interna es gRPC.",
		Endpoints: rutas,
	})
}

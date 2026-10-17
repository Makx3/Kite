package httpapi

import "net/http"

// health responde la vitalidad del gateway y el estado de conectividad de
// cada dependencia. No invoca a los servicios (no añade carga): se apoya en
// el estado de la conexión gRPC, que se actualiza con el tráfico real.
func (h *Handler) health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status": "ok",
		"backends": map[string]string{
			"users":         h.users.State(),
			"relationships": h.relationships.State(),
			"notifications": h.notifications.State(),
		},
	})
}

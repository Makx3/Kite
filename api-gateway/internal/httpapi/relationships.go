package httpapi

import (
	"context"
	"net/http"
)

// follow atiende POST /users/{user_id}/follow. El seguidor viene del
// encabezado X-User-Id y el seguido del path, tal como define el diseño.
func (h *Handler) follow(w http.ResponseWriter, r *http.Request) {
	followerID := currentUserFromHeader(r)
	if followerID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el usuario que ejecuta la acción")
		return
	}

	follow, err := h.relationships.Follow(r.Context(), followerID, r.PathValue("user_id"))
	if err != nil {
		respondBackendError(w, h.logger, "relationships", err)
		return
	}
	writeJSON(w, http.StatusCreated, toFollowDTO(follow))
}

// unfollow atiende DELETE /users/{user_id}/follow.
func (h *Handler) unfollow(w http.ResponseWriter, r *http.Request) {
	followerID := currentUserFromHeader(r)
	if followerID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el usuario que ejecuta la acción")
		return
	}

	if err := h.relationships.Unfollow(r.Context(), followerID, r.PathValue("user_id")); err != nil {
		respondBackendError(w, h.logger, "relationships", err)
		return
	}
	// Sin cuerpo: la arista ya no existe.
	w.WriteHeader(http.StatusNoContent)
}

// followers atiende GET /users/{user_id}/followers.
func (h *Handler) followers(w http.ResponseWriter, r *http.Request) {
	h.listFollowIDs(w, r, h.relationships.Followers)
}

// following atiende GET /users/{user_id}/following.
func (h *Handler) following(w http.ResponseWriter, r *http.Request) {
	h.listFollowIDs(w, r, h.relationships.Following)
}

// listFollowIDs comparte la lógica de los dos listados paginados del
// grafo: leen limit/offset, delegan y normalizan la página.
func (h *Handler) listFollowIDs(
	w http.ResponseWriter,
	r *http.Request,
	fetch func(ctx context.Context, userID string, limit, offset int32) ([]string, int32, error),
) {
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	ids, total, err := fetch(r.Context(), r.PathValue("user_id"), limit, offset)
	if err != nil {
		respondBackendError(w, h.logger, "relationships", err)
		return
	}
	if ids == nil {
		// Una página vacía es una lista vacía, no null.
		ids = []string{}
	}
	writeJSON(w, http.StatusOK, userIDsDTO{UserIDs: ids, Total: total})
}

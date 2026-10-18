package httpapi

import (
	"net/http"

	postspb "github.com/Makx3/Kite/api-gateway/pb/posts"
)

// feedDTO es la página del muro personal, agregada en el gateway.
type feedDTO struct {
	Posts       []*postDTO `json:"posts"`
	Total       int32      `json:"total"`
	LastUpdated string     `json:"last_updated"`
}

// getFeed atiende GET /feed. Es la segunda agregación del gateway:
// Feed devuelve los identificadores del muro (ordenados, con total y
// última actualización) y Posts hidrata el contenido de esa página.
// Si Posts falla, el feed entero falla: sin contenido no hay muro
// útil. Los identificadores que Posts no reconoce se omiten (un post
// borrado no debe romper la lectura del resto).
func (h *Handler) getFeed(w http.ResponseWriter, r *http.Request) {
	userID := currentUserFromHeader(r)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el dueño del muro")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	ids, total, lastUpdated, err := h.feed.UserFeed(r.Context(), userID, limit, offset)
	if err != nil {
		respondBackendError(w, h.logger, "feed", err)
		return
	}
	if len(ids) == 0 {
		writeJSON(w, http.StatusOK, feedDTO{Posts: []*postDTO{}, Total: total, LastUpdated: lastUpdated})
		return
	}

	posts, err := h.posts.GetByIDs(r.Context(), ids)
	if err != nil {
		respondBackendError(w, h.logger, "posts", err)
		return
	}

	// Se reordena según el orden del muro (Feed), no según la respuesta
	// de Posts: el feed es quien define la cronología visible.
	byID := make(map[string]*postspb.Post, len(posts))
	for _, p := range posts {
		byID[p.GetPostId()] = p
	}
	out := make([]*postDTO, 0, len(ids))
	for _, id := range ids {
		if p, ok := byID[id]; ok {
			out = append(out, toPostDTO(p))
		} else {
			h.logger.Warn("gateway: post del feed no encontrado en Posts",
				"post_id", id, "user_id", userID)
		}
	}
	writeJSON(w, http.StatusOK, feedDTO{Posts: out, Total: total, LastUpdated: lastUpdated})
}

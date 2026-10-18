package httpapi

import "net/http"

// likePost atiende POST /posts/{post_id}/like. El usuario viene del
// encabezado X-User-Id. El gateway resuelve el autor del post con
// Posts.GetPost y se lo entrega a Likes: así la notificación no
// depende de que Likes conozca a Posts (el diseño lo exige).
func (h *Handler) likePost(w http.ResponseWriter, r *http.Request) {
	userID := currentUserFromHeader(r)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el usuario que reacciona")
		return
	}
	postID := r.PathValue("post_id")

	author, err := h.posts.Get(r.Context(), postID)
	if err != nil {
		// Un post inexistente o un Posts caído se traduce aquí mismo:
		// 404 o 503 sin siquiera llamar a Likes.
		respondBackendError(w, h.logger, "posts", err)
		return
	}

	resp, err := h.likes.Like(r.Context(), postID, userID, author.GetUserId())
	if err != nil {
		respondBackendError(w, h.logger, "likes", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{
		"like_id": resp.GetLikeId(),
		"status":  resp.GetStatus(),
	})
}

// unlikePost atiende DELETE /posts/{post_id}/like.
func (h *Handler) unlikePost(w http.ResponseWriter, r *http.Request) {
	userID := currentUserFromHeader(r)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el usuario que reacciona")
		return
	}

	if err := h.likes.Unlike(r.Context(), r.PathValue("post_id"), userID); err != nil {
		respondBackendError(w, h.logger, "likes", err)
		return
	}
	// Sin cuerpo: la reacción ya no existe.
	w.WriteHeader(http.StatusNoContent)
}

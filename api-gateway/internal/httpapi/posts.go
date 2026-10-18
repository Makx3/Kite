package httpapi

import (
	"net/http"
	"strings"

	postspb "github.com/Makx3/Kite/api-gateway/pb/posts"
)

// postDTO es la representación REST de una publicación, con los
// campos del contrato del diseño.
type postDTO struct {
	PostID    string `json:"post_id"`
	UserID    string `json:"user_id"`
	Content   string `json:"content"`
	CreatedAt string `json:"created_at"`
}

func toPostDTO(p *postspb.Post) *postDTO {
	if p == nil {
		return nil
	}
	return &postDTO{
		PostID:    p.GetPostId(),
		UserID:    p.GetUserId(),
		Content:   p.GetContent(),
		CreatedAt: formatTS(p.GetCreatedAt()),
	}
}

// createdDTO es la respuesta de creación de una publicación.
type createdDTO struct {
	PostID    string `json:"post_id"`
	Status    string `json:"status"`
	CreatedAt string `json:"created_at"`
}

// createPost atiende POST /posts. El autor viene del encabezado
// X-User-Id y el contenido del cuerpo, tal como define el diseño.
func (h *Handler) createPost(w http.ResponseWriter, r *http.Request) {
	userID := currentUserFromHeader(r)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el autor de la publicación")
		return
	}
	var body struct {
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	post, err := h.posts.Create(r.Context(), userID, strings.TrimSpace(body.Content))
	if err != nil {
		respondBackendError(w, h.logger, "posts", err)
		return
	}
	writeJSON(w, http.StatusCreated, createdDTO{
		PostID:    post.GetPostId(),
		Status:    "created",
		CreatedAt: formatTS(post.GetCreatedAt()),
	})
}

// getPost atiende GET /posts/{post_id}: delega en Posts.GetPost.
func (h *Handler) getPost(w http.ResponseWriter, r *http.Request) {
	post, err := h.posts.Get(r.Context(), r.PathValue("post_id"))
	if err != nil {
		respondBackendError(w, h.logger, "posts", err)
		return
	}
	writeJSON(w, http.StatusOK, toPostDTO(post))
}

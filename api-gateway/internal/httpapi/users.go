package httpapi

import "net/http"

// createUser atiende POST /users: delega en Users.CreateUser.
func (h *Handler) createUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		DisplayName string `json:"display_name"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, err := h.users.Create(r.Context(), body.Username, body.Email, body.DisplayName)
	if err != nil {
		respondBackendError(w, h.logger, "users", err)
		return
	}
	writeJSON(w, http.StatusCreated, toUserDTO(user))
}

// getUser atiende GET /users/{user_id}: delega en Users.GetUser.
func (h *Handler) getUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.users.Get(r.Context(), r.PathValue("user_id"))
	if err != nil {
		respondBackendError(w, h.logger, "users", err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(user))
}

// updateUser atiende PATCH /users/{user_id}: delega en Users.UpdateUser.
// Los campos ausentes no se tocan; el username no es editable.
func (h *Handler) updateUser(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DisplayName *string `json:"display_name"`
		Email       *string `json:"email"`
	}
	if err := decodeJSON(r, &body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid_body", err.Error())
		return
	}

	user, err := h.users.Update(r.Context(), r.PathValue("user_id"), body.DisplayName, body.Email)
	if err != nil {
		respondBackendError(w, h.logger, "users", err)
		return
	}
	writeJSON(w, http.StatusOK, toUserDTO(user))
}

package httpapi

import (
	"context"
	"net/http"
	"strings"
	"sync"

	notificationspb "github.com/Makx3/Kite/api-gateway/pb/notifications"
)

// notificationsDTO es la página de la bandeja de alertas.
type notificationsDTO struct {
	Notifications []*notificationDTO `json:"notifications"`
	Total         int32              `json:"total"`
}

// listNotifications atiende GET /notifications. Es el punto de agregación
// del gateway: pide las alertas a Notifications y luego hidrata el emisor
// de cada una con Users, en dos llamadas a servicios distintos y sin que
// ninguno lea la base de datos del otro.
func (h *Handler) listNotifications(w http.ResponseWriter, r *http.Request) {
	userID := currentUserFromHeader(r)
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"falta el encabezado X-User-Id con el dueño de la bandeja")
		return
	}
	limit, offset, err := pagination(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_query", err.Error())
		return
	}

	items, total, err := h.notifications.Unread(r.Context(), userID, limit, offset)
	if err != nil {
		respondBackendError(w, h.logger, "notifications", err)
		return
	}

	sources := h.loadSourceUsers(r.Context(), items)

	out := make([]*notificationDTO, 0, len(items))
	for _, n := range items {
		dto := toNotificationDTO(n)
		dto.SourceUser = sources[n.GetSourceUserId()]
		out = append(out, dto)
	}
	writeJSON(w, http.StatusOK, notificationsDTO{Notifications: out, Total: total})
}

// loadSourceUsers resuelve el emisor de cada alerta llamando a Users una
// sola vez por identificador distinto. Si Users falla para alguno, se omite
// ese emisor y se registra: la bandeja se entrega igual (degradación
// elegante) en vez de fallar completa por un campo accesorio.
func (h *Handler) loadSourceUsers(ctx context.Context, items []*notificationspb.Notification) map[string]*userDTO {
	unique := make(map[string]struct{}, len(items))
	for _, n := range items {
		if id := n.GetSourceUserId(); id != "" {
			unique[id] = struct{}{}
		}
	}
	if len(unique) == 0 {
		return nil
	}

	var (
		mu  sync.Mutex
		wg  sync.WaitGroup
		out = make(map[string]*userDTO, len(unique))
	)
	for id := range unique {
		wg.Add(1)
		go func(id string) {
			defer wg.Done()
			u, err := h.users.Get(ctx, id)
			if err != nil {
				h.logger.Warn("gateway: no se pudo hidratar el emisor de la alerta",
					"source_user_id", id, "error", err)
				return
			}
			mu.Lock()
			out[id] = toUserDTO(u)
			mu.Unlock()
		}(id)
	}
	wg.Wait()
	return out
}

// markNotificationRead atiende POST /notifications/{notification_id}/read.
func (h *Handler) markNotificationRead(w http.ResponseWriter, r *http.Request) {
	// El diseño ubica user_id en el cuerpo para esta operación; además se
	// acepta el encabezado, uniforme con el resto de la API.
	userID := currentUserFromHeader(r)
	if userID == "" {
		var body struct {
			UserID string `json:"user_id"`
		}
		if err := decodeJSON(r, &body); err == nil {
			userID = strings.TrimSpace(body.UserID)
		}
	}
	if userID == "" {
		writeError(w, http.StatusBadRequest, "missing_user",
			"indica el usuario en el encabezado X-User-Id o en el campo user_id")
		return
	}

	n, err := h.notifications.MarkRead(r.Context(), userID, r.PathValue("notification_id"))
	if err != nil {
		respondBackendError(w, h.logger, "notifications", err)
		return
	}
	writeJSON(w, http.StatusOK, toNotificationDTO(n))
}

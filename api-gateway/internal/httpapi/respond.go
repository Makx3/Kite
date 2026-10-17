package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// errorBody es el cuerpo uniforme de error: un identificador estable para
// el cliente y un mensaje legible.
type errorBody struct {
	Error   string `json:"error"`
	Message string `json:"message"`
}

// writeJSON serializa v con el código indicado. Un v nil solo envía la
// cabecera (útil para respuestas sin cuerpo).
func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// La cabecera ya se envió: solo queda registrar el fallo.
		slog.Error("gateway: no se pudo serializar la respuesta", "error", err)
	}
}

// writeError responde con el cuerpo de error uniforme.
func writeError(w http.ResponseWriter, code int, slug, message string) {
	writeJSON(w, code, errorBody{Error: slug, Message: message})
}

// respondBackendError traduce el error de un servicio de negocio a una
// respuesta HTTP. Los errores de validación del backend se propagan con su
// mensaje (son útiles y no revelan infraestructura); los fallos internos y
// de red se registran con detalle y se responden de forma genérica. Un
// servicio caído produce 503, nunca una caída del gateway.
func respondBackendError(w http.ResponseWriter, logger *slog.Logger, backend string, err error) {
	if errors.Is(err, context.DeadlineExceeded) {
		logger.Warn("gateway: tiempo agotado", "servicio", backend, "error", err)
		writeError(w, http.StatusGatewayTimeout, "deadline_exceeded",
			"el servicio de destino no respondió a tiempo")
		return
	}

	switch status.Code(err) {
	case codes.InvalidArgument:
		writeError(w, http.StatusBadRequest, "invalid_argument", status.Convert(err).Message())
	case codes.NotFound:
		writeError(w, http.StatusNotFound, "not_found", status.Convert(err).Message())
	case codes.AlreadyExists:
		writeError(w, http.StatusConflict, "already_exists", status.Convert(err).Message())
	case codes.DeadlineExceeded:
		logger.Warn("gateway: tiempo agotado", "servicio", backend, "error", err)
		writeError(w, http.StatusGatewayTimeout, "deadline_exceeded",
			"el servicio de destino no respondió a tiempo")
	case codes.Unavailable:
		logger.Warn("gateway: servicio no disponible", "servicio", backend, "error", err)
		writeError(w, http.StatusServiceUnavailable, "service_unavailable",
			"el servicio de destino no está disponible")
	case codes.Canceled:
		writeError(w, http.StatusRequestTimeout, "canceled", "la petición fue cancelada")
	default:
		logger.Error("gateway: error del servicio", "servicio", backend, "error", err)
		writeError(w, http.StatusBadGateway, "bad_gateway",
			"el servicio de destino devolvió un error")
	}
}

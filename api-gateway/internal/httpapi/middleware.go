package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusRecorder captura el código de estado para el log de acceso.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Write asegura que un handler que solo escribe cuerpo cuente como 200.
func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// chain aplica los middlewares dejando el primero como el más externo.
func chain(h http.Handler, mws ...func(http.Handler) http.Handler) http.Handler {
	for i := len(mws) - 1; i >= 0; i-- {
		h = mws[i](h)
	}
	return h
}

// recovery convierte un panic en un 500: el gateway nunca se cae por una
// petición defectuosa.
func recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("gateway: panic recuperado",
						"metodo", r.Method,
						"ruta", r.URL.Path,
						"panic", rec,
						"stack", string(debug.Stack()),
					)
					writeError(w, http.StatusInternalServerError, "internal_error",
						"error interno del gateway")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// accessLog registra método, ruta, estado y duración de cada petición.
func accessLog(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w}
			next.ServeHTTP(rec, r)
			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			logger.Info("http",
				"metodo", r.Method,
				"ruta", r.URL.Path,
				"estado", rec.status,
				"duracion_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// timeout acota la petición completa propagando el plazo por el contexto;
// es la red de seguridad para handlers que encadenan varias llamadas gRPC.
func timeout(d time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), d)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

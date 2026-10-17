package httpapi

import "net/http"

// uniformNotFound envuelve el enrutador para que el 404 del contrato no dependa
// de lo que haga Go por defecto. Cuando ningún patrón coincide, ServeMux
// responde "404 page not found" en texto plano, lo que contradiría el cuerpo de
// error uniforme {"error","message"} que documenta el README.
//
// No basta con registrar un comodín "/": ese patrón se tragaría los caminos
// cuyo método no coincide y convertiría los 405 en 200. En cambio, cuando
// mux.Handler no encuentra patrón el enrutador no ejecuta ninguna ruta propia
// —solo responde 404, 405 (método incorrecto, con encabezado Allow) o una
// redirección de 301—, así que se puede capturar esa respuesta sin efectos
// secundarios: el 404 se normaliza y el resto (405, 301) se reemite tal cual.
func uniformNotFound(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		capturada := &capture{header: make(http.Header)}
		mux.ServeHTTP(capturada, r)

		if capturada.status == http.StatusNotFound {
			writeError(w, http.StatusNotFound, "not_found",
				"la ruta no existe: "+r.Method+" "+r.URL.Path)
			return
		}

		for clave, valores := range capturada.header {
			w.Header()[clave] = valores
		}
		w.WriteHeader(capturada.status)
		if len(capturada.body) > 0 {
			_, _ = w.Write(capturada.body)
		}
	})
}

// capture es un ResponseWriter en memoria, solo para reemitir la respuesta que
// el enrutador generó por su cuenta.
type capture struct {
	header http.Header
	body   []byte
	status int
}

func (c *capture) Header() http.Header { return c.header }

func (c *capture) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}

func (c *capture) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	c.body = append(c.body, p...)
	return len(p), nil
}

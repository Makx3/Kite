package httpapi

import (
	"net/http"
	"strings"
)

// routeFallbacks envuelve el enrutador para cubrir los dos huecos que quedan
// entre lo que enruta Go por defecto y lo que promete el contrato REST del
// gateway.
//
//  1. Barra final. ServeMux no considera /users/{id}/ la misma ruta que
//     /users/{id}: el comodín {user_id} no cruza la barra y la petición
//     termina en 404. Como es la variante que cualquiera teclea en el
//     navegador, se vuelve a despachar sin la barra final. Es una reescritura
//     interna y no una redirección: así funciona igual con POST, PATCH o
//     DELETE, donde un 301 podría reenviarse como GET y perder el cuerpo.
//
//  2. Cuerpo del 404. Cuando ningún patrón coincide, ServeMux responde
//     "404 page not found" en texto plano, lo que contradiría el cuerpo de
//     error uniforme {"error","message"} que documentan el README y el
//     documento técnico.
//
// No sirve registrar un comodín "/": ese patrón se tragaría los caminos cuyo
// método no coincide y convertiría los 405 en 200 (GET /users respondería 200).
// En cambio, cuando mux.Handler no encuentra patrón el enrutador no ejecuta
// ninguna de nuestras rutas —solo responde 404, 405 (con encabezado Allow) o
// una redirección de 301—, así que se puede capturar esa respuesta sin efectos
// secundarios: el 404 se normaliza y el resto se reemite tal cual.
func routeFallbacks(mux *http.ServeMux) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, pattern := mux.Handler(r); pattern != "" {
			mux.ServeHTTP(w, r)
			return
		}

		// La barra final no cambia el recurso: se reintenta sin ella.
		if sinBarra, ok := sinBarraFinal(mux, r); ok {
			reescrita := r.Clone(r.Context())
			reescrita.URL.Path = sinBarra
			mux.ServeHTTP(w, reescrita)
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

// sinBarraFinal devuelve el camino sin la barra final cuando esa variante sí
// tiene ruta registrada para este método. La raíz ("/") se deja intacta.
func sinBarraFinal(mux *http.ServeMux, r *http.Request) (string, bool) {
	camino := r.URL.Path
	if camino == "/" || !strings.HasSuffix(camino, "/") {
		return "", false
	}
	candidato := strings.TrimSuffix(camino, "/")
	sonda := r.Clone(r.Context())
	sonda.URL.Path = candidato
	if _, pattern := mux.Handler(sonda); pattern == "" {
		return "", false
	}
	return candidato, true
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

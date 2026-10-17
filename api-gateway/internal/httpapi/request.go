package httpapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"strings"
)

// decodeJSON lee el cuerpo JSON de la petición. Un cuerpo vacío se acepta
// como objeto vacío para que la validación del servicio de destino sea la
// que produzca el mensaje de error.
func decodeJSON(r *http.Request, dst any) error {
	if r.Body == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return nil
		}
		return fmt.Errorf("el cuerpo debe ser JSON válido")
	}
	return nil
}

// currentUserFromHeader devuelve el usuario que actúa. En la primera
// entrega no hay autenticación: el encabezado X-User-Id es la identidad
// declarada por el cliente y se documenta como tal.
func currentUserFromHeader(r *http.Request) string {
	return strings.TrimSpace(r.Header.Get("X-User-Id"))
}

// pagination lee limit y offset de la query. Ausentes valen 0, que el
// servicio de destino interpreta como su valor por defecto.
func pagination(r *http.Request) (limit, offset int32, err error) {
	if limit, err = int32Query(r, "limit"); err != nil {
		return 0, 0, err
	}
	if offset, err = int32Query(r, "offset"); err != nil {
		return 0, 0, err
	}
	return limit, offset, nil
}

func int32Query(r *http.Request, name string) (int32, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 0 || n > math.MaxInt32 {
		return 0, fmt.Errorf("%s debe ser un entero no negativo", name)
	}
	return int32(n), nil
}

package services

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// timestampLayouts mapea los formatos que expone la configuración al layout
// de Go equivalente, junto al ancho al que hay que rellenar con ceros.
var timestampLayouts = map[string]struct {
	layout string
	width  int
}{
	"YYYYMMDD": {"20060102", 8},
	"HHMMSS":   {"150405", 6},
	"HHMM":     {"1504", 4},
}

// BuildDerivedTimestamp combina el campo de fecha y el de hora del documento
// en un time.Time UTC. Devuelve ok=false si algún campo falta, no parsea, o
// la fecha resultante no existe — el llamador ingiere el documento igual,
// sin el campo derivado: un timestamp inconstruible es un dato incompleto,
// no un dato inválido.
func BuildDerivedTimestamp(cfg models.DerivedTimestampConfig, doc bson.M) (time.Time, bool) {
	dateSpec, ok := timestampLayouts[cfg.DateFormat]
	if !ok {
		return time.Time{}, false
	}
	timeSpec, ok := timestampLayouts[cfg.TimeFormat]
	if !ok {
		return time.Time{}, false
	}

	datePart, ok := padNumericField(doc[cfg.DateField], dateSpec.width)
	if !ok {
		return time.Time{}, false
	}
	timePart, ok := padNumericField(doc[cfg.TimeField], timeSpec.width)
	if !ok {
		return time.Time{}, false
	}

	parsed, err := time.ParseInLocation(dateSpec.layout+timeSpec.layout, datePart+timePart, time.UTC)
	if err != nil {
		return time.Time{}, false
	}
	return parsed, true
}

// padNumericField normaliza el valor a un string de exactamente width
// dígitos. El relleno de ceros es la razón de ser de esta función: la hora
// se guarda como número, así que "021517" vuelve como 21517 y sin rellenar
// se interpretaría como 21:51:07 en vez de 02:15:17.
func padNumericField(raw interface{}, width int) (string, bool) {
	var s string
	switch v := raw.(type) {
	case nil:
		return "", false
	case string:
		s = strings.TrimSpace(v)
	case int32:
		s = strconv.FormatInt(int64(v), 10)
	case int64:
		s = strconv.FormatInt(v, 10)
	case int:
		s = strconv.Itoa(v)
	case float64:
		s = strconv.FormatInt(int64(v), 10)
	default:
		return "", false
	}
	if s == "" {
		return "", false
	}
	if _, err := strconv.ParseInt(s, 10, 64); err != nil {
		return "", false
	}
	if len(s) > width {
		return "", false
	}
	return fmt.Sprintf("%0*s", width, s), true
}

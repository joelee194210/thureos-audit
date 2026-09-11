package services

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/thureos/compliance/internal/models"
	"golang.org/x/text/unicode/norm"
)

// monitorAliasMaxLen topea el alias — el LLM lo escribe en cada tool
// call, y un nombre de monitor largo no aporta legibilidad extra.
const monitorAliasMaxLen = 40

// stripDiacritics descompone en NFD y descarta las marcas combinantes
// (categoría Mn), que es como se quitan tildes y diéresis sin tabla
// manual: "Región" -> "Region".
func stripDiacritics(s string) string {
	var b strings.Builder
	for _, r := range norm.NFD.String(s) {
		if unicode.Is(unicode.Mn, r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// monitorAlias deriva el identificador corto que el LLM usa para nombrar
// un monitor en query_monitor_data. Determinista para un mismo nombre.
// Devuelve "" si el nombre no tiene ningún carácter alfanumérico — el
// llamador (buildMonitorAliases) se encarga del respaldo.
func monitorAlias(name string) string {
	var b strings.Builder
	prevUnderscore := false
	for _, r := range strings.ToLower(stripDiacritics(name)) {
		switch {
		case r < 128 && (unicode.IsLetter(r) || unicode.IsDigit(r)):
			b.WriteRune(r)
			prevUnderscore = false
		case !prevUnderscore && b.Len() > 0:
			// Colapsa cualquier corrida de separadores en un solo "_", y
			// nunca abre el alias con uno.
			b.WriteRune('_')
			prevUnderscore = true
		}
	}
	alias := strings.Trim(b.String(), "_")
	if len(alias) > monitorAliasMaxLen {
		alias = strings.Trim(alias[:monitorAliasMaxLen], "_")
	}
	return alias
}

// buildMonitorAliases arma el mapa alias -> monitor de una conversación.
// Es la ÚNICA fuente de verdad de qué monitores puede tocar el LLM en un
// turno: se construye a partir de los monitores de la conversación y de
// nada más. Ver Global Constraints (autorización).
//
// El orden de `monitors` define qué monitor se queda con el alias base
// cuando hay colisión, así que debe ser estable (el de conv.MonitorIDs).
func buildMonitorAliases(monitors []models.Monitor) map[string]*models.Monitor {
	aliases := make(map[string]*models.Monitor, len(monitors))
	for i := range monitors {
		base := monitorAlias(monitors[i].Name)
		if base == "" {
			base = fmt.Sprintf("monitor_%d", i+1)
		}
		alias := base
		for n := 2; ; n++ {
			if _, taken := aliases[alias]; !taken {
				break
			}
			alias = fmt.Sprintf("%s_%d", base, n)
		}
		aliases[alias] = &monitors[i]
	}
	return aliases
}

// resolveQueryMonitor traduce el alias que mandó el LLM al monitor real.
//
// ESTA ES LA FRONTERA DE AUTORIZACIÓN del chatbot multi-monitor: el mapa
// `aliases` se construye solo a partir de conv.MonitorIDs, así que un
// alias que no está ahí no tiene forma de convertirse en una consulta.
// Nunca se acepta un ObjectID crudo: si el LLM manda un hex, cae acá como
// alias desconocido igual que cualquier otro texto.
//
// Importa porque los datos que el LLM lee son archivos subidos por
// usuarios: una celda con una inyección de prompt puede pedirle que
// consulte otro monitor, y este chequeo es lo que hace que ese pedido no
// llegue a ningún lado.
func resolveQueryMonitor(aliases map[string]*models.Monitor, alias string) (*models.Monitor, error) {
	if monitor, ok := aliases[alias]; ok {
		return monitor, nil
	}
	valid := make([]string, 0, len(aliases))
	for a := range aliases {
		valid = append(valid, a)
	}
	sort.Strings(valid)
	return nil, fmt.Errorf(
		"el monitor %q no existe en esta conversación; los disponibles son: %s",
		alias, strings.Join(valid, ", "),
	)
}

// chatToolIterations y chatAskTimeout escalan con la cantidad de
// monitores: una pregunta cruzada necesita al menos una llamada por
// monitor antes de poder empezar a razonar, así que las cotas fijas
// pensadas para un solo monitor se agotaban sin haber respondido.
const (
	chatBaseToolIterations       = 5
	chatToolIterationsPerMonitor = 3
	chatMaxToolIterations        = 20

	chatBaseAskTimeout       = 60 * time.Second
	chatAskTimeoutPerMonitor = 30 * time.Second
	chatMaxAskTimeout        = 240 * time.Second
)

func chatToolIterations(monitorCount int) int {
	n := chatBaseToolIterations + chatToolIterationsPerMonitor*monitorCount
	if n > chatMaxToolIterations {
		return chatMaxToolIterations
	}
	return n
}

func chatAskTimeout(monitorCount int) time.Duration {
	d := chatBaseAskTimeout + chatAskTimeoutPerMonitor*time.Duration(monitorCount)
	if d > chatMaxAskTimeout {
		return chatMaxAskTimeout
	}
	return d
}

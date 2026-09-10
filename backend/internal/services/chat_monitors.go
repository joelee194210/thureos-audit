package services

import (
	"fmt"
	"strings"
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

package services

import "fmt"

// Moldeado de los resultados de las sources de un artefacto antes de
// renderizarlos. Todo lo de acá es puro: entra data, sale data.

// artifactMonitorColumn es el nombre de la columna discriminadora que
// concatSources agrega cuando hay más de una fuente.
const artifactMonitorColumn = "_monitor"

// projectRows recorta cada fila a las columnas pedidas. Sin columnas,
// devuelve las filas como vinieron.
//
// Hace falta porque QueryData devuelve el documento COMPLETO de Mongo:
// sin proyección, una tabla sobre filas crudas vuelca todos los campos,
// incluido el _id.
func projectRows(rows []map[string]interface{}, columns []string) []map[string]interface{} {
	if len(columns) == 0 {
		return rows
	}
	out := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		projected := make(map[string]interface{}, len(columns))
		for _, col := range columns {
			// Una columna que no está en la fila se omite, no se inventa
			// como nil: así la tabla no muestra una columna fantasma.
			if v, ok := row[col]; ok {
				projected[col] = v
			}
		}
		out = append(out, projected)
	}
	return out
}

// concatSources apila los resultados de varias fuentes agregando una
// columna que dice de cuál vino cada fila. Es la forma correcta para una
// TABLA comparativa.
func concatSources(results [][]map[string]interface{}, labels []string) []map[string]interface{} {
	out := []map[string]interface{}{}
	for i, rows := range results {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		for _, row := range rows {
			merged := make(map[string]interface{}, len(row)+1)
			for k, v := range row {
				merged[k] = v
			}
			merged[artifactMonitorColumn] = label
			out = append(out, merged)
		}
	}
	return out
}

// pivotSourceLabels calcula el nombre de columna que cada fuente usa al
// pivotear, evitando dos colisiones que pivotSources sufría en silencio:
//
//   - Dos fuentes con el mismo label (o ambas sin label, "" == "") se
//     pisarían: la segunda sobreescribiría el valor de la primera. Es el
//     caso típico de comparar el mismo monitor en dos ventanas de tiempo
//     distintas — mismo alias, mismo label salvo que el LLM ponga Label
//     explícito, cosa que nada exige.
//   - Un label igual a xKey pisaría pivoted[xKey], borrando el valor de x
//     de la fila.
//
// Se desambigua agregando el índice de la fuente (1-based) al nombre en
// colisión: la primera fuente que usa un label se queda con el nombre tal
// cual, y solo las siguientes en colisión se sufijan — así un artefacto
// sin fuentes duplicadas no cambia de comportamiento. El sufijo crece
// (" (n)", " (n) (n)", ...) hasta encontrar un nombre libre; con como
// máximo n fuentes eso siempre termina.
func pivotSourceLabels(labels []string, xKey string, n int) []string {
	used := map[string]bool{xKey: true}
	out := make([]string, n)
	for i := 0; i < n; i++ {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		candidate := label
		for used[candidate] {
			candidate = fmt.Sprintf("%s (%d)", candidate, i+1)
		}
		used[candidate] = true
		out[i] = candidate
	}
	return out
}

// pivotSources arma una fila por valor de xKey y una columna por fuente.
// Es la forma correcta para un GRÁFICO con una serie por monitor:
// concatenar daría una sola serie con las x repetidas, que Recharts apila
// mal en vez de dibujar dos series.
//
// Una fuente que no tiene dato para una x deja la celda ausente (nil al
// serializar): Recharts corta ahí la línea, que es lo correcto — no había
// dato, y un cero mentiría.
//
// Conserva el orden en que aparecen las x: el pipeline de agregación ya
// las ordenó por valor, y reordenar acá tiraría ese trabajo.
//
// Agrupa por la representación textual de x (fmt.Sprint), no por x en sí:
// xKey lo elige el LLM, y si apunta a un campo con un valor no comparable
// (un arreglo o un documento anidado tal como vino de Mongo), usarlo
// directo como llave de un map[interface{}] paniquearía ("hash of
// unhashable type"). fmt.Sprint no necesita que el valor sea hasheable y
// alcanza para agrupar igual con igual; la fila de salida sigue llevando
// el x original, no su representación textual.
func pivotSources(results [][]map[string]interface{}, labels []string, xKey, valueKey string) []map[string]interface{} {
	type xRow struct {
		x    interface{}
		vals map[string]interface{}
	}

	effectiveLabels := pivotSourceLabels(labels, xKey, len(results))

	order := []string{}
	byX := map[string]*xRow{}

	for i, rows := range results {
		label := effectiveLabels[i]
		for _, row := range rows {
			x, ok := row[xKey]
			if !ok {
				continue
			}
			key := fmt.Sprint(x)
			entry, seen := byX[key]
			if !seen {
				entry = &xRow{x: x, vals: map[string]interface{}{xKey: x}}
				byX[key] = entry
				order = append(order, key)
			}
			entry.vals[label] = row[valueKey]
		}
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, key := range order {
		out = append(out, byX[key].vals)
	}
	return out
}

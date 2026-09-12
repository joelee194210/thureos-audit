package services

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
func pivotSources(results [][]map[string]interface{}, labels []string, xKey, valueKey string) []map[string]interface{} {
	order := []interface{}{}
	byX := map[interface{}]map[string]interface{}{}

	for i, rows := range results {
		label := ""
		if i < len(labels) {
			label = labels[i]
		}
		for _, row := range rows {
			x, ok := row[xKey]
			if !ok {
				continue
			}
			pivoted, seen := byX[x]
			if !seen {
				pivoted = map[string]interface{}{xKey: x}
				byX[x] = pivoted
				order = append(order, x)
			}
			pivoted[label] = row[valueKey]
		}
	}

	out := make([]map[string]interface{}, 0, len(order))
	for _, x := range order {
		out = append(out, byX[x])
	}
	return out
}

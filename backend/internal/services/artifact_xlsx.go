package services

import (
	"bytes"
	"fmt"
	"sort"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// artifactSheetName: Excel topea el nombre de hoja en 31 caracteres y
// prohíbe varios símbolos, así que se usa uno fijo en vez del título.
const artifactSheetName = "Datos"

// BuildArtifactXLSX arma el Excel de un artefacto ya ejecutado.
//
// Los números van como números (no como texto) porque lo primero que hace
// quien baja el archivo es sumarlos u ordenarlos, y una columna de texto
// no se deja. Las fechas van con formato de fecha, por lo mismo.
func BuildArtifactXLSX(art *models.ChatArtifact, run ArtifactRun) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	index, err := f.NewSheet(artifactSheetName)
	if err != nil {
		return nil, fmt.Errorf("creando la hoja: %w", err)
	}
	f.SetActiveSheet(index)
	_ = f.DeleteSheet("Sheet1")

	columns := artifactColumns(art, run.Data)

	headerStyle, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return nil, fmt.Errorf("creando el estilo de cabecera: %w", err)
	}
	// Formato de fecha propio en vez del built-in 22 ("m/d/yy h:mm", orden
	// estadounidense): la app formatea en es-CO en todos lados, día antes
	// que mes.
	dateFmt := "dd/mm/yyyy hh:mm"
	dateStyle, err := f.NewStyle(&excelize.Style{CustomNumFmt: &dateFmt})
	if err != nil {
		return nil, fmt.Errorf("creando el estilo de fecha: %w", err)
	}

	// Cabecera: el rótulo de presentación si lo hay, si no la clave.
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(artifactSheetName, cell, artifactColumnLabel(art, col))
		_ = f.SetCellStyle(artifactSheetName, cell, cell, headerStyle)
	}

	for r, row := range run.Data {
		for c, col := range columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			value := artifactCellValue(row[col])
			_ = f.SetCellValue(artifactSheetName, cell, value)
			if _, isDate := value.(time.Time); isDate {
				_ = f.SetCellStyle(artifactSheetName, cell, cell, dateStyle)
			}
		}
	}

	// Fecha de corrida: el spec de exportación la exige en los tres
	// formatos —sin ella, dos descargas del mismo artefacto no se pueden
	// distinguir por vigencia, y en cumplimiento eso no es cosmético. Va
	// después de los datos para no correr la cabecera de la tabla.
	footerRow := len(run.Data) + 2
	labelCell, _ := excelize.CoordinatesToCellName(1, footerRow)
	valueCell, _ := excelize.CoordinatesToCellName(2, footerRow)
	_ = f.SetCellValue(artifactSheetName, labelCell, "Fecha de corrida:")
	_ = f.SetCellStyle(artifactSheetName, labelCell, labelCell, headerStyle)
	_ = f.SetCellValue(artifactSheetName, valueCell, run.RanAt)
	_ = f.SetCellStyle(artifactSheetName, valueCell, valueCell, dateStyle)

	// Ancho de columna por contenido: se calcula al final, sobre la hoja
	// ya completa, porque AutoFitColWidth lee lo que ya está escrito.
	// El rango cubre como mínimo A:B para que la fila de "Fecha de
	// corrida:" también entre.
	lastIdx := len(columns) - 1
	if lastIdx < 1 {
		lastIdx = 1
	}
	widthRange := fmt.Sprintf("A:%s", colLetter(lastIdx))
	if err := f.AutoFitColWidth(artifactSheetName, widthRange); err != nil {
		return nil, fmt.Errorf("ajustando el ancho de columnas: %w", err)
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serializando el xlsx: %w", err)
	}
	return buf.Bytes(), nil
}

// artifactCellValue normaliza los tipos que excelize no reconoce en su
// propio type switch (que es por tipo EXACTO, no por interfaz ni por tipo
// subyacente) para que no caigan a su default y salgan como texto:
//
//   - primitive.DateTime: el tipo fecha nativo del driver de Mongo. Las
//     filas de un artefacto salen de bsonToMaps, o sea de documentos
//     decodificados tal cual —una fecha ahí NO es time.Time— y
//     primitive.DateTime es un int64 con nombre propio: no matchea ni
//     "case int64" ni "case time.Time" (mismo criterio que ya distingue
//     red_flag_report.go al formatear). Sin esto caería al default y se
//     imprimiría como texto plano de milisegundos.
//   - primitive.ObjectID: implementa Stringer, así que SÍ cae al default
//     de excelize sin error — pero su String() devuelve `ObjectID("...")`
//     con comillas y el nombre del tipo incluidos, no el hex limpio. Una
//     tabla sobre filas crudas (sin proyección) puede volcar el _id tal
//     cual, y esa cadena no es lo que nadie espera ver en una celda.
func artifactCellValue(value interface{}) interface{} {
	switch v := value.(type) {
	case primitive.DateTime:
		return v.Time()
	case primitive.ObjectID:
		return v.Hex()
	default:
		return value
	}
}

// artifactColumns decide qué columnas van y en qué orden: la proyección
// si el artefacto la declara, si no la unión de las claves de todas las
// filas, ORDENADA — el recorrido de un mapa en Go es aleatorio y sin
// ordenar el archivo saldría distinto en cada descarga.
func artifactColumns(art *models.ChatArtifact, rows []map[string]interface{}) []string {
	if art.ChartSpec != nil && len(art.ChartSpec.Columns) > 0 {
		return art.ChartSpec.Columns
	}
	seen := map[string]bool{}
	columns := []string{}
	for _, row := range rows {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				columns = append(columns, k)
			}
		}
	}
	sort.Strings(columns)
	return columns
}

func artifactColumnLabel(art *models.ChatArtifact, column string) string {
	if art.ChartSpec != nil {
		if label, ok := art.ChartSpec.Labels[column]; ok && label != "" {
			return label
		}
	}
	return column
}

func colLetter(index int) string {
	name, _ := excelize.ColumnNumberToName(index + 1)
	return name
}

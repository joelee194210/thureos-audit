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
	dateStyle, err := f.NewStyle(&excelize.Style{NumFmt: 22}) // d/m/yyyy h:mm
	if err != nil {
		return nil, fmt.Errorf("creando el estilo de fecha: %w", err)
	}

	// Cabecera: el rótulo de presentación si lo hay, si no la clave.
	for i, col := range columns {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		_ = f.SetCellValue(artifactSheetName, cell, artifactColumnLabel(art, col))
		_ = f.SetCellStyle(artifactSheetName, cell, cell, headerStyle)
		_ = f.SetColWidth(artifactSheetName, colLetter(i), colLetter(i), 18)
	}

	for r, row := range run.Data {
		for c, col := range columns {
			cell, _ := excelize.CoordinatesToCellName(c+1, r+2)
			value := row[col]
			// Las filas salen de bsonToMaps, es decir de documentos de
			// Mongo decodificados tal cual: una fecha ahí NO es time.Time,
			// es primitive.DateTime (el tipo fecha nativo del driver;
			// mismo criterio que ya usa red_flag_report.go al formatear).
			// excelize.SetCellValue hace un type switch por tipo exacto, y
			// primitive.DateTime — un int64 con nombre propio — no
			// matchea ni "case int64" ni "case time.Time": sin este caso
			// caería al default y se imprimiría como texto plano de
			// milisegundos.
			if dt, ok := value.(primitive.DateTime); ok {
				value = dt.Time()
			}
			if t, ok := value.(time.Time); ok {
				_ = f.SetCellValue(artifactSheetName, cell, t)
				_ = f.SetCellStyle(artifactSheetName, cell, cell, dateStyle)
				continue
			}
			// excelize infiere el tipo del valor de Go: un float64 entra
			// como número, un string como texto. Por eso NO se convierte
			// nada a string antes de escribir.
			_ = f.SetCellValue(artifactSheetName, cell, value)
		}
	}

	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		return nil, fmt.Errorf("serializando el xlsx: %w", err)
	}
	return buf.Bytes(), nil
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

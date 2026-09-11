package services

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-pdf/fpdf"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Fuente embebida en el binario: el core "Helvetica" de fpdf usa cp1252 y
// desfigura acentos/eñes si se le pasan strings UTF-8 tal cual (que es como
// llegan todos los textos de este informe). DejaVu Sans sí soporta UTF-8
// completo — se registra como "DejaVu" en RenderRedFlagReportPDF.
//
//go:embed fonts/dejavu-sans-condensed.ttf
var dejaVuRegularTTF []byte

//go:embed fonts/dejavu-sans-condensed-bold.ttf
var dejaVuBoldTTF []byte

//go:embed fonts/dejavu-sans-condensed-oblique.ttf
var dejaVuObliqueTTF []byte

const reportFont = "DejaVu"

// Colores del tema claro de frontend/src/styles/tokens/thureos-tokens.css
// (marca "compliance"). El backend no tiene DOM/CSS para resolverlos como
// hace frontend/src/lib/pdf-tokens.ts en el navegador, así que se
// transcriben a mano — si los tokens cambian, hay que actualizar esto.
type reportRGB struct{ r, g, b int }

var (
	rColorInk     = reportRGB{4, 24, 44}     // --fg-default
	rColorMuted   = reportRGB{66, 88, 110}   // --fg-muted
	rColorLine    = reportRGB{203, 216, 227} // --border-default
	rColorSurface = reportRGB{238, 244, 249} // --bg-surface-2
	rColorAccent  = reportRGB{0, 72, 176}    // --accent-fg (brand=compliance, light)
)

var riskColors = map[models.Severity]struct{ fg, bg reportRGB }{
	models.SeverityLow:      {reportRGB{11, 107, 74}, reportRGB{228, 246, 239}},
	models.SeverityMedium:   {reportRGB{140, 80, 6}, reportRGB{255, 244, 224}},
	models.SeverityHigh:     {reportRGB{168, 31, 27}, reportRGB{253, 236, 235}},
	models.SeverityCritical: {reportRGB{255, 255, 255}, reportRGB{142, 31, 27}},
}

var severityLabels = map[models.Severity]string{
	models.SeverityLow:      "Baja",
	models.SeverityMedium:   "Media",
	models.SeverityHigh:     "Alta",
	models.SeverityCritical: "Crítica",
}

var redFlagStatusLabels = map[models.RedFlagStatus]string{
	models.RedFlagNew:          "Nueva",
	models.RedFlagAcknowledged: "Reconocida",
	models.RedFlagResolved:     "Resuelta",
	models.RedFlagDismissed:    "Descartada",
}

var monthsEs = [...]string{"ene", "feb", "mar", "abr", "may", "jun", "jul", "ago", "sep", "oct", "nov", "dic"}

func formatDateEs(t time.Time) string {
	t = t.Local()
	return fmt.Sprintf("%d %s %d, %02d:%02d", t.Day(), monthsEs[t.Month()-1], t.Year(), t.Hour(), t.Minute())
}

const (
	reportMarginMM = 14.0
	reportHeaderH  = 24.0
	reportMaxCols  = 8
)

func setTextRGB(pdf *fpdf.Fpdf, c reportRGB) { pdf.SetTextColor(c.r, c.g, c.b) }
func setFillRGB(pdf *fpdf.Fpdf, c reportRGB) { pdf.SetFillColor(c.r, c.g, c.b) }
func setDrawRGB(pdf *fpdf.Fpdf, c reportRGB) { pdf.SetDrawColor(c.r, c.g, c.b) }

// redFlagSummaryRows arma la ficha de la bandera roja.
//
// El bloque de agregación se omite cuando no hay función de agregación: las
// banderas de las reglas de velocidad son agrupadas (RedFlagType "velocity")
// pero sin AggField/AggFunction/AggValue/Threshold, y renderizarlo igual
// imprimía "Agregación: ( ) por tarjeta", "Valor: 0" y "Umbral: 0" en el PDF
// que es el artefacto probatorio del caso.
// El grupo sí se conserva: para una regla de velocidad es la entidad
// señalada (la tarjeta), y es un dato correcto.
func redFlagSummaryRows(rf *models.RedFlag) [][2]string {
	isAggregate := rf.RedFlagType.EsAgrupada()
	// El label es "qué tipo de regla generó esto", no "está agrupada": una
	// bandera de velocidad también agrupa (EsAgrupada() == true) pero no es
	// una agregación, así que el label sale del tipo mismo.
	tipo := "Por coincidencia"
	switch rf.RedFlagType {
	case models.RedFlagTypeVelocity:
		tipo = "Velocidad"
	case models.RedFlagTypeAggregate:
		tipo = "Agregada"
	}
	summary := [][2]string{
		{"Identificador", rf.ID.Hex()},
		{"Monitor", rf.MonitorName},
		{"Regla", rf.RuleName},
		{"Tipo", tipo},
		{"Detectada", formatDateEs(rf.CreatedAt)},
		{"Coincidencias", strconv.Itoa(rf.MatchCount)},
	}
	hasAggDetail := isAggregate && rf.AggFunction != ""
	if hasAggDetail {
		summary = append(summary, [2]string{
			"Agregación",
			fmt.Sprintf("%s(%s) por %s", strings.ToUpper(rf.AggFunction), orDash(rf.AggField), orDash(rf.GroupByField)),
		})
	}
	if isAggregate && rf.GroupByValue != "" {
		summary = append(summary, [2]string{"Grupo", rf.GroupByValue})
	}
	if hasAggDetail {
		summary = append(summary, [2]string{"Valor", formatReportFloat(rf.AggValue)})
		summary = append(summary, [2]string{"Umbral", formatReportFloat(rf.Threshold)})
		if rf.Threshold != 0 {
			pct := (rf.AggValue / rf.Threshold) * 100
			summary = append(summary, [2]string{"Sobre el umbral", fmt.Sprintf("%.0f%%", pct)})
		}
	}
	return summary
}

// RenderRedFlagReportPDF arma el informe PDF de una bandera roja recién
// creada. Replica las secciones de frontend/src/lib/red-flag-report.ts —
// salvo la bitácora de auditoría, que todavía no existe en el momento de
// creación (se deja una nota en su lugar).
func RenderRedFlagReportPDF(rf *models.RedFlag) ([]byte, error) {
	generatedAt := time.Now()

	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddUTF8FontFromBytes(reportFont, "", dejaVuRegularTTF)
	pdf.AddUTF8FontFromBytes(reportFont, "B", dejaVuBoldTTF)
	pdf.AddUTF8FontFromBytes(reportFont, "I", dejaVuObliqueTTF)
	pdf.SetMargins(reportMarginMM, reportHeaderH+6, reportMarginMM)
	pdf.SetAutoPageBreak(true, 20)
	pageW, pageH := pdf.GetPageSize()
	contentW := pageW - reportMarginMM*2
	pdf.AliasNbPages("{nb}")

	pdf.SetHeaderFunc(func() {
		setFillRGB(pdf, rColorSurface)
		pdf.Rect(0, 0, pageW, reportHeaderH, "F")
		pdf.SetFont(reportFont, "B", 11)
		setTextRGB(pdf, rColorAccent)
		pdf.SetXY(reportMarginMM, 5)
		pdf.CellFormat(contentW, 6, "Thureos Compliance", "", 1, "L", false, 0, "")
		pdf.SetFont(reportFont, "", 9)
		setTextRGB(pdf, rColorMuted)
		pdf.SetXY(reportMarginMM, 11)
		pdf.CellFormat(contentW/2, 6, "Informe de bandera roja", "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 6, formatDateEs(generatedAt), "", 1, "R", false, 0, "")
		pdf.SetXY(reportMarginMM, reportHeaderH+6)
	})

	pdf.SetFooterFunc(func() {
		setDrawRGB(pdf, rColorLine)
		pdf.SetLineWidth(0.2)
		pdf.Line(reportMarginMM, pageH-12, pageW-reportMarginMM, pageH-12)
		pdf.SetFont(reportFont, "I", 7.5)
		setTextRGB(pdf, rColorMuted)
		pdf.SetXY(reportMarginMM, pageH-9)
		pdf.CellFormat(contentW/2, 5, fmt.Sprintf("Bandera roja %s", rf.ID.Hex()), "", 0, "L", false, 0, "")
		pdf.CellFormat(contentW/2, 5, fmt.Sprintf("Página %d de {nb}", pdf.PageNo()), "", 0, "R", false, 0, "")
	})

	pdf.AddPage()

	// ---- Título y chips ----
	pdf.SetFont(reportFont, "B", 15)
	setTextRGB(pdf, rColorInk)
	pdf.MultiCell(contentW, 6.5, rf.RuleName, "", "L", false)
	pdf.Ln(4)

	risk, ok := riskColors[rf.Severity]
	if !ok {
		risk = struct{ fg, bg reportRGB }{rColorInk, rColorSurface}
	}
	y := pdf.GetY()
	chipX := reportMarginMM
	chipX = drawReportChip(pdf, fmt.Sprintf("Severidad: %s", severityLabels[rf.Severity]), risk.fg, risk.bg, chipX, y)
	statusLabel := redFlagStatusLabels[rf.Status]
	if statusLabel == "" {
		statusLabel = redFlagStatusLabels[models.RedFlagNew]
	}
	drawReportChip(pdf, fmt.Sprintf("Estado: %s", statusLabel), rColorInk, rColorSurface, chipX, y)
	pdf.SetXY(reportMarginMM, y+8)

	if rf.Message != "" {
		pdf.SetFont(reportFont, "", 9.5)
		setTextRGB(pdf, rColorMuted)
		pdf.MultiCell(contentW, 4.6, rf.Message, "", "L", false)
		pdf.Ln(2)
	}

	// ---- Ficha ----
	drawReportSectionTitle(pdf, "Ficha de la bandera roja", contentW)

	drawReportKeyValueTable(pdf, redFlagSummaryRows(rf), contentW)
	pdf.Ln(4)

	// ---- Registros afectados ----
	drawReportSectionTitle(pdf, "Registros afectados", contentW)
	if len(rf.MatchedRecords) == 0 {
		drawReportNote(pdf, "Esta bandera roja no tiene registros asociados.", contentW)
	} else {
		columns := recordColumns(rf.MatchedRecords[0])
		if len(columns) > reportMaxCols {
			columns = columns[:reportMaxCols]
		}
		drawReportRecordsTable(pdf, columns, rf.MatchedRecords, contentW)
	}
	pdf.Ln(4)

	// ---- Bitácora ----
	drawReportSectionTitle(pdf, "Bitácora de auditoría", contentW)
	drawReportNote(pdf,
		"Este informe se generó automáticamente al detectarse la alerta; "+
			"todavía no hay acciones registradas en la bitácora.",
		contentW)

	var buf bytes.Buffer
	if err := pdf.Output(&buf); err != nil {
		return nil, fmt.Errorf("rendering red flag report: %w", err)
	}
	return buf.Bytes(), nil
}

func drawReportChip(pdf *fpdf.Fpdf, text string, fg, bg reportRGB, x, y float64) float64 {
	pdf.SetFont(reportFont, "", 8.5)
	w := pdf.GetStringWidth(text) + 6
	setFillRGB(pdf, bg)
	pdf.RoundedRect(x, y-4.2, w, 6.2, 1.2, "1234", "F")
	setTextRGB(pdf, fg)
	pdf.SetXY(x, y-4.2)
	pdf.CellFormat(w, 6.2, text, "", 0, "C", false, 0, "")
	return x + w + 3
}

func drawReportSectionTitle(pdf *fpdf.Fpdf, title string, contentW float64) {
	pdf.SetX(reportMarginMM)
	y := pdf.GetY()
	pdf.SetFont(reportFont, "B", 10.5)
	setTextRGB(pdf, rColorInk)
	pdf.CellFormat(contentW, 6, title, "", 1, "L", false, 0, "")
	setDrawRGB(pdf, rColorLine)
	pdf.SetLineWidth(0.2)
	pdf.Line(reportMarginMM, y+6.5, reportMarginMM+contentW, y+6.5)
	pdf.SetXY(reportMarginMM, y+9)
}

func drawReportNote(pdf *fpdf.Fpdf, text string, contentW float64) {
	pdf.SetX(reportMarginMM)
	pdf.SetFont(reportFont, "I", 8.5)
	setTextRGB(pdf, rColorMuted)
	pdf.MultiCell(contentW, 4.2, text, "", "L", false)
}

func drawReportKeyValueTable(pdf *fpdf.Fpdf, rows [][2]string, contentW float64) {
	labelW := 45.0
	for _, row := range rows {
		y := pdf.GetY()
		pdf.SetFont(reportFont, "", 9)
		setTextRGB(pdf, rColorMuted)
		pdf.SetXY(reportMarginMM, y)
		pdf.CellFormat(labelW, 6, row[0], "", 0, "L", false, 0, "")
		pdf.SetFont(reportFont, "B", 9)
		setTextRGB(pdf, rColorInk)
		pdf.SetXY(reportMarginMM+labelW, y)
		pdf.MultiCell(contentW-labelW, 6, row[1], "", "L", false)
	}
}

func drawReportRecordsTable(pdf *fpdf.Fpdf, columns []string, records []map[string]interface{}, contentW float64) {
	colW := contentW / float64(len(columns))

	pdf.SetFont(reportFont, "B", 7)
	setFillRGB(pdf, rColorSurface)
	setTextRGB(pdf, rColorInk)
	pdf.SetX(reportMarginMM)
	for i, col := range columns {
		ln := 0
		if i == len(columns)-1 {
			ln = 1
		}
		pdf.CellFormat(colW, 6, col, "1", ln, "L", true, 0, "")
	}

	pdf.SetFont(reportFont, "", 7)
	setTextRGB(pdf, rColorInk)
	for _, rec := range records {
		pdf.SetX(reportMarginMM)
		for i, col := range columns {
			ln := 0
			if i == len(columns)-1 {
				ln = 1
			}
			pdf.CellFormat(colW, 6, formatRecordValue(rec[col]), "1", ln, "L", false, 0, "")
		}
	}
}

// recordColumns devuelve las claves del registro sin el prefijo "_",
// ordenadas alfabéticamente. Un map de Go no conserva orden de inserción
// como sí lo hace un objeto de JS — se ordena para que el resultado sea
// determinístico entre corridas, aunque no coincida exactamente con el
// orden que muestra el informe generado en el navegador.
func recordColumns(record map[string]interface{}) []string {
	columns := make([]string, 0, len(record))
	for k := range record {
		if !strings.HasPrefix(k, "_") {
			columns = append(columns, k)
		}
	}
	sort.Strings(columns)
	return columns
}

func formatRecordValue(v interface{}) string {
	switch val := v.(type) {
	case nil:
		return "—"
	case string:
		if val == "" {
			return "—"
		}
		if t, err := time.Parse(time.RFC3339, val); err == nil {
			return formatDateEs(t)
		}
		return val
	case bool:
		if val {
			return "Sí"
		}
		return "No"
	case float64:
		return formatReportFloat(val)
	case float32:
		return formatReportFloat(float64(val))
	case int:
		return strconv.Itoa(val)
	case int32:
		return strconv.Itoa(int(val))
	case int64:
		return strconv.FormatInt(val, 10)
	case primitive.DateTime:
		return formatDateEs(val.Time())
	case time.Time:
		return formatDateEs(val)
	default:
		b, err := json.Marshal(val)
		if err != nil {
			return fmt.Sprintf("%v", val)
		}
		return string(b)
	}
}

// formatReportFloat da un formato legible simple (separador de miles con
// coma, hasta dos decimales) — no intenta replicar exactamente el
// Intl.NumberFormat("es-CO") del informe en el navegador.
func formatReportFloat(v float64) string {
	neg := v < 0
	if neg {
		v = -v
	}
	intPart := int64(v)
	frac := v - float64(intPart)

	digits := strconv.FormatInt(intPart, 10)
	var grouped []byte
	n := len(digits)
	for i := 0; i < n; i++ {
		if i > 0 && (n-i)%3 == 0 {
			grouped = append(grouped, ',')
		}
		grouped = append(grouped, digits[i])
	}
	result := string(grouped)
	if frac > 0.005 {
		result += strings.TrimPrefix(fmt.Sprintf("%.2f", frac), "0")
	}
	if neg {
		result = "-" + result
	}
	return result
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

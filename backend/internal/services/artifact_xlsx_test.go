package services

import (
	"bytes"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func artifactWithRows() (*models.ChatArtifact, ArtifactRun) {
	art := &models.ChatArtifact{
		Type:  models.ChatArtifactTable,
		Title: "Ventas por región",
		ChartSpec: &models.ChartSpec{
			Columns: []string{"region", "monto"},
			Labels:  map[string]string{"region": "Región", "monto": "Monto total"},
		},
	}
	run := ArtifactRun{
		RanAt: time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC),
		Data: []map[string]interface{}{
			{"region": "Caribe", "monto": 1500.5},
			{"region": "Andina", "monto": 2300.0},
		},
	}
	return art, run
}

func TestBuildArtifactXLSX_UsaLosRotulosComoCabecera(t *testing.T) {
	art, run := artifactWithRows()
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}

	f, err := excelize.OpenReader(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("el archivo generado no es un xlsx válido: %v", err)
	}
	sheet := f.GetSheetName(0)

	// La cabecera muestra el rótulo, no la clave interna.
	if got, _ := f.GetCellValue(sheet, "A1"); got != "Región" {
		t.Errorf("A1 = %q, quiero %q", got, "Región")
	}
	if got, _ := f.GetCellValue(sheet, "B1"); got != "Monto total" {
		t.Errorf("B1 = %q, quiero %q", got, "Monto total")
	}
}

// Los montos tienen que entrar como NÚMEROS: si van como texto, Excel no
// los suma ni los ordena, que es lo primero que hace quien baja el
// archivo.
func TestBuildArtifactXLSX_LosNumerosSonNumeros(t *testing.T) {
	art, run := artifactWithRows()
	data, _ := BuildArtifactXLSX(art, run)
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	cellType, err := f.GetCellType(sheet, "B2")
	if err != nil {
		t.Fatalf("no pude leer el tipo de B2: %v", err)
	}
	if cellType == excelize.CellTypeSharedString || cellType == excelize.CellTypeInlineString {
		t.Errorf("B2 quedó como texto (%v); un monto debe ser numérico", cellType)
	}
}

func TestBuildArtifactXLSX_SinFilasNoFalla(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "Vacío"}
	if _, err := BuildArtifactXLSX(art, ArtifactRun{RanAt: time.Now()}); err != nil {
		t.Fatalf("un artefacto sin filas debe generar un archivo vacío, no un error: %v", err)
	}
}

// Las filas de un artefacto NO vienen de código Go que arme time.Time a
// mano: salen de bsonToMaps, que decodifica documentos de Mongo tal cual
// los devuelve el driver. Una fecha ahí decodifica a primitive.DateTime,
// no a time.Time (ver red_flag_report.go, que ya distingue los dos casos
// al formatear). excelize.SetCellValue hace un type switch por tipo
// EXACTO: primitive.DateTime es un int64 con nombre propio y no matchea
// ni "case int64" ni "case time.Time", así que sin un caso dedicado cae
// al default y se imprime como texto plano de milisegundos — silencioso,
// y exactamente lo que esta tarea prohíbe para las fechas.
func TestBuildArtifactXLSX_FechaDeMongoQuedaComoFecha(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "T"}
	fecha := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	run := ArtifactRun{
		RanAt: time.Now(),
		Data:  []map[string]interface{}{{"fecha": primitive.NewDateTimeFromTime(fecha)}},
	}
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	cellType, err := f.GetCellType(sheet, "A2")
	if err != nil {
		t.Fatalf("no pude leer el tipo de A2: %v", err)
	}
	if cellType == excelize.CellTypeSharedString || cellType == excelize.CellTypeInlineString {
		t.Errorf("A2 quedó como texto (%v); una fecha de Mongo (primitive.DateTime) debe quedar como fecha", cellType)
	}
	got, err := f.GetCellValue(sheet, "A2")
	if err != nil {
		t.Fatalf("no pude leer el valor de A2: %v", err)
	}
	if got == "1773568800000" {
		t.Errorf("A2 = %q: quedó como el entero crudo de milisegundos, no como fecha", got)
	}
}

// FIX ROUND 1, arreglo 1: el spec de exportación lo exige para los tres
// formatos —«el documento lleva impresa la fecha de corrida para que no
// haya ambigüedad sobre qué se está mirando»— y aplica igual al XLSX. Sin
// esto, dos descargas del mismo artefacto no se pueden distinguir por
// vigencia.
func TestBuildArtifactXLSX_ImprimeFechaDeCorrida(t *testing.T) {
	art, run := artifactWithRows() // 2 filas de datos
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	// Fila 1 = cabecera, filas 2-3 = datos, fila 4 = la de la fecha de
	// corrida: va después de los datos para no correr la cabecera de la
	// tabla.
	if got, _ := f.GetCellValue(sheet, "A4"); got != "Fecha de corrida:" {
		t.Errorf("A4 = %q, quiero el rótulo de la fecha de corrida", got)
	}
	got, err := f.GetCellValue(sheet, "B4")
	if err != nil {
		t.Fatalf("no pude leer B4: %v", err)
	}
	if want := "15/03/2026 10:00"; got != want {
		t.Errorf("B4 = %q, quiero %q (RanAt formateada)", got, want)
	}
	// Tiene que ser una fecha real, no el texto armado a mano: así se
	// puede seguir comparando/ordenando si alguien la copia a otra hoja.
	cellType, err := f.GetCellType(sheet, "B4")
	if err != nil {
		t.Fatalf("no pude leer el tipo de B4: %v", err)
	}
	if cellType == excelize.CellTypeSharedString || cellType == excelize.CellTypeInlineString {
		t.Errorf("B4 quedó como texto (%v); la fecha de corrida debe ser una fecha real", cellType)
	}
}

// FIX ROUND 1, arreglo 3: primitive.ObjectID implementa Stringer, así que
// SÍ cae al default de excelize sin error — pero su String() devuelve
// `ObjectID("507f...")`, con comillas y el nombre del tipo incluidos, no
// el hex limpio. Una tabla sobre filas crudas (sin proyección, el
// escenario que nombra el spec) puede volcar el _id tal cual.
func TestBuildArtifactXLSX_ObjectIDSaleComoHexLimpio(t *testing.T) {
	id := primitive.NewObjectID()
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "T"}
	run := ArtifactRun{
		RanAt: time.Now(),
		Data:  []map[string]interface{}{{"id": id}},
	}
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	got, _ := f.GetCellValue(sheet, "A2")
	if want := id.Hex(); got != want {
		t.Errorf("A2 = %q, quiero el hex limpio %q (no %q)", got, want, id.String())
	}
}

// FIX ROUND 1, arreglo 2: el spec pide ancho de columna por contenido, no
// un ancho fijo para todas. Una columna con contenido mucho más largo que
// otra tiene que terminar más ancha.
func TestBuildArtifactXLSX_AnchoDeColumnaPorContenido(t *testing.T) {
	art := &models.ChatArtifact{
		Type:      models.ChatArtifactTable,
		Title:     "T",
		ChartSpec: &models.ChartSpec{Columns: []string{"corto", "largo"}},
	}
	run := ArtifactRun{
		RanAt: time.Now(),
		Data: []map[string]interface{}{
			{"corto": "x", "largo": "un valor bastante más largo que el de la otra columna"},
		},
	}
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)

	cortoWidth, err := f.GetColWidth(sheet, "A")
	if err != nil {
		t.Fatalf("no pude leer el ancho de A: %v", err)
	}
	largoWidth, err := f.GetColWidth(sheet, "B")
	if err != nil {
		t.Fatalf("no pude leer el ancho de B: %v", err)
	}
	if largoWidth <= cortoWidth {
		t.Errorf("ancho columna larga = %v, columna corta = %v; la más larga debería ser más ancha", largoWidth, cortoWidth)
	}
}

func TestBuildArtifactXLSX_SinProyeccionUsaLasClavesDeLasFilas(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable, Title: "T"}
	run := ArtifactRun{
		RanAt: time.Now(),
		Data:  []map[string]interface{}{{"b": 2, "a": 1}},
	}
	data, err := BuildArtifactXLSX(art, run)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	f, _ := excelize.OpenReader(bytes.NewReader(data))
	sheet := f.GetSheetName(0)
	// Las claves se ordenan para que el archivo sea reproducible: el
	// recorrido de un mapa en Go es aleatorio.
	if got, _ := f.GetCellValue(sheet, "A1"); got != "a" {
		t.Errorf("A1 = %q, quiero %q (columnas ordenadas)", got, "a")
	}
}

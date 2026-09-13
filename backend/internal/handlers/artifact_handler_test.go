package handlers

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/mongo"
)

// El defecto que este test fija: el nombre de archivo de la exportación
// solo sacaba comillas y CR/LF, sin tocar los caracteres no-ASCII. En un
// producto en español eso NO es el caso borde — es el caso común: casi
// cualquier título real trae tildes, eñes o ¿¡. xlsxContentDisposition
// debe seguir produciendo un filename= ASCII legible (sin mojibake) Y un
// filename*= con el prefijo UTF-8 y el título completo, para que el
// navegador que lo lea muestre el nombre correcto en vez de la versión
// degradada.
func TestXLSXContentDisposition_TituloConAcentosYComillas(t *testing.T) {
	got := xlsxContentDisposition(`Análisis "Región" Caribe`)
	want := `attachment; filename="Analisis Region Caribe.xlsx"; filename*=UTF-8''An%C3%A1lisis%20Regi%C3%B3n%20Caribe.xlsx`
	if got != want {
		t.Errorf("xlsxContentDisposition(título con acentos y comillas) =\n%q\nse esperaba\n%q", got, want)
	}
}

func TestXLSXContentDisposition_SoloSimbolos(t *testing.T) {
	// Un título sin ningún carácter ASCII reconocible (aquí, solo
	// símbolos) no puede degradar a un filename="" vacío — eso rompe la
	// descarga en algunos clientes. Debe caer al nombre genérico.
	got := xlsxContentDisposition("★彡")
	want := `attachment; filename="artefacto.xlsx"; filename*=UTF-8''%E2%98%85%E5%BD%A1.xlsx`
	if got != want {
		t.Errorf("xlsxContentDisposition(solo símbolos) =\n%q\nse esperaba\n%q", got, want)
	}
}

func TestSanitizeXLSXFilename_SacaComillas(t *testing.T) {
	got := sanitizeXLSXFilename(`Reporte "mensual"`)
	want := "Reporte mensual"
	if got != want {
		t.Errorf("sanitizeXLSXFilename(con comillas) = %q, se esperaba %q", got, want)
	}
}

func TestSanitizeXLSXFilename_SacaCRLF(t *testing.T) {
	// c.Set() de fasthttp ya descarta \r y \n de cualquier valor de
	// cabecera antes de escribirlo — esa es la defensa real contra
	// inyección de CRLF. Esta función los saca igual, cinturón y
	// tirantes, pero lo que hace el trabajo que fasthttp NO hace es la
	// comilla del test de arriba.
	got := sanitizeXLSXFilename("Reporte\r\nmensual")
	want := "Reportemensual"
	if got != want {
		t.Errorf("sanitizeXLSXFilename(con CRLF) = %q, se esperaba %q", got, want)
	}
}

func TestAsciiXLSXFilename_QuitaTildesYEnes(t *testing.T) {
	got := asciiXLSXFilename("Región Bogotá Peña Ñoño")
	want := "Region Bogota Pena Nono"
	if got != want {
		t.Errorf("asciiXLSXFilename(tildes y eñes) = %q, se esperaba %q", got, want)
	}
}

func TestAsciiXLSXFilename_SoloSimbolosCaeAlGenerico(t *testing.T) {
	got := asciiXLSXFilename("★彡")
	want := "artefacto"
	if got != want {
		t.Errorf("asciiXLSXFilename(solo símbolos) = %q, se esperaba %q", got, want)
	}
}

// HALLAZGO 4 (revisión de rama): load() colapsaba CUALQUIER error de
// GetByID en 404. Un blip del replica set le decía a la biblioteca que el
// artefacto ya no existe —y RunArtifact, veinte líneas más abajo, es
// escrupuloso con lo contrario—. Solo "no hay documento" es 404; todo lo
// demás es 500. Lo que NO cambia: un artefacto de otro usuario sigue
// siendo 404, indistinguible de "no existe" (eso se decide en load, contra
// art.UserID, no acá).
func TestArtifactLoadStatus_SinDocumentoEs404(t *testing.T) {
	if got := artifactLoadStatus(mongo.ErrNoDocuments); got != fiber.StatusNotFound {
		t.Errorf("status = %d, quiero 404", got)
	}
	// GetByID envuelve con %w: el centinela tiene que seguir reconociéndose
	// a través del wrap, que es como llega en producción.
	envuelto := fmt.Errorf("artefacto %s no encontrado: %w", "abc", mongo.ErrNoDocuments)
	if got := artifactLoadStatus(envuelto); got != fiber.StatusNotFound {
		t.Errorf("status (envuelto) = %d, quiero 404", got)
	}
}

func TestArtifactLoadStatus_ErrorTransitorioEs500(t *testing.T) {
	for _, err := range []error{
		context.DeadlineExceeded,
		errors.New("server selection error: no reachable servers"),
		fmt.Errorf("decodificando el artefacto: %w", errors.New("bson: invalid")),
	} {
		if got := artifactLoadStatus(err); got != fiber.StatusInternalServerError {
			t.Errorf("artifactLoadStatus(%v) = %d, quiero 500: un fallo transitorio no es 'no existe'", err, got)
		}
	}
}

// HALLAZGO 3 (revisión de rama): ExportXLSX solo miraba CachedData, así
// que una instantánea con sus filas en chartSpec.data —chart/table con
// data inline y sin sources, que ParseArtifact acepta, Ask persiste con id
// real y availableFormats ofrece exportar a Excel— devolvía 500 con los
// datos ahí al lado.
func TestSnapshotData_CaeAlDataInlineDeLaInstantanea(t *testing.T) {
	art := &models.ChatArtifact{
		Type: models.ChatArtifactTable,
		ChartSpec: &models.ChartSpec{
			Data: []map[string]interface{}{{"region": "Caribe"}},
		},
	}
	got := snapshotData(art)
	if len(got) != 1 || got[0]["region"] != "Caribe" {
		t.Fatalf("snapshotData = %v, quiero las filas de chartSpec.data", got)
	}
}

// La caché de una corrida real gana: es más nueva que los datos con los
// que el artefacto nació.
func TestSnapshotData_PrefiereLaCacheSobreElDataInline(t *testing.T) {
	art := &models.ChatArtifact{
		Type:       models.ChatArtifactTable,
		CachedData: []map[string]interface{}{{"region": "Andina"}},
		ChartSpec:  &models.ChartSpec{Data: []map[string]interface{}{{"region": "Caribe"}}},
	}
	if got := snapshotData(art); len(got) != 1 || got[0]["region"] != "Andina" {
		t.Fatalf("snapshotData = %v, quiero la caché", got)
	}
}

// `custom` es código con sus datos embebidos: no hay tabla que volcar. Es
// el único caso que queda sin filas, y por eso el handler responde 4xx
// —un pedido que no aplica— y no 500 ni 409 (ese código ya significa "el
// monitor de origen desapareció" y el frontend lo traduce con ese texto).
func TestSnapshotData_CustomNoTieneFilas(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactCustom, Code: "<p>hola</p>"}
	if got := snapshotData(art); got != nil {
		t.Fatalf("snapshotData = %v, quiero nil para un custom", got)
	}
}

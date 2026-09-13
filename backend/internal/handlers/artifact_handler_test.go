package handlers

import "testing"

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

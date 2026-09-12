package services

import (
	"testing"
)

func TestProjectRows_RecortaYOrdena(t *testing.T) {
	rows := []map[string]interface{}{
		{"_id": "x", "monto": 100, "region": "Caribe", "extra": "ruido"},
	}
	got := projectRows(rows, []string{"region", "monto"})

	if len(got) != 1 || len(got[0]) != 2 {
		t.Fatalf("quiero 1 fila de 2 columnas, tengo %v", got)
	}
	if got[0]["region"] != "Caribe" || got[0]["monto"] != 100 {
		t.Errorf("valores incorrectos: %v", got[0])
	}
	if _, hay := got[0]["_id"]; hay {
		t.Error("_id no fue proyectado y no debería aparecer")
	}
}

func TestProjectRows_SinColumnasDevuelveTodo(t *testing.T) {
	rows := []map[string]interface{}{{"a": 1, "b": 2}}
	got := projectRows(rows, nil)
	if len(got[0]) != 2 {
		t.Errorf("sin proyección deben venir todas las columnas: %v", got[0])
	}
}

func TestProjectRows_ColumnaInexistenteSeOmite(t *testing.T) {
	rows := []map[string]interface{}{{"a": 1}}
	got := projectRows(rows, []string{"a", "no_existe"})
	if _, hay := got[0]["no_existe"]; hay {
		t.Error("una columna que no está en la fila no debe inventarse")
	}
}

func TestConcatSources_ApilaConDiscriminador(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 10}},
		{{"_id": "Andina", "aggValue": 20}},
	}
	got := concatSources(results, []string{"Bancolombia", "Davivienda"})

	if len(got) != 2 {
		t.Fatalf("quiero 2 filas, tengo %d", len(got))
	}
	if got[0]["_monitor"] != "Bancolombia" || got[1]["_monitor"] != "Davivienda" {
		t.Errorf("falta o está mal el discriminador: %v", got)
	}
}

// Concatenar un chart multi-fuente daría UNA serie con las x repetidas,
// que Recharts apila mal. Una serie por monitor exige pivotear.
func TestPivotSources_UnaColumnaPorFuente(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}, {"_id": "Andina", "aggValue": 80}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	got := pivotSources(results, []string{"Bancolombia", "Davivienda"}, "_id", "aggValue")

	if len(got) != 2 {
		t.Fatalf("quiero 2 filas (una por valor de x), tengo %d: %v", len(got), got)
	}

	byX := map[interface{}]map[string]interface{}{}
	for _, row := range got {
		byX[row["_id"]] = row
	}
	if byX["Caribe"]["Bancolombia"] != 100 || byX["Caribe"]["Davivienda"] != 60 {
		t.Errorf("fila Caribe incorrecta: %v", byX["Caribe"])
	}
	if byX["Andina"]["Bancolombia"] != 80 {
		t.Errorf("fila Andina incorrecta: %v", byX["Andina"])
	}
	// Una fuente sin dato para esa x deja nil: Recharts corta la línea,
	// que es lo correcto — no había dato, no es un cero.
	if v, hay := byX["Andina"]["Davivienda"]; hay && v != nil {
		t.Errorf("Davivienda no tiene dato para Andina, debería ser nil: %v", v)
	}
}

func TestPivotSources_ConservaElOrdenDeAparicion(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "z", "aggValue": 1}, {"_id": "a", "aggValue": 2}},
	}
	got := pivotSources(results, []string{"S"}, "_id", "aggValue")
	if got[0]["_id"] != "z" || got[1]["_id"] != "a" {
		t.Errorf("el pivote debe conservar el orden en que llegaron las x (el pipeline ya ordenó): %v", got)
	}
}

// FIX ROUND 1 / ARREGLO 2: como ParseArtifact reescribe src.Monitor con
// el alias normalizado, dos fuentes sobre el MISMO monitor (comparar
// 2024 contra 2025, por ejemplo) producen el mismo label salvo que el
// LLM ponga Label explícito — nada lo exige. Antes del arreglo, la
// segunda fuente pisaba en silencio la columna de la primera y el
// gráfico perdía una serie entera.
func TestPivotSources_LabelsDuplicadosNoSePisan(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	got := pivotSources(results, []string{"tx", "tx"}, "_id", "aggValue")

	if len(got) != 1 {
		t.Fatalf("quiero 1 fila (un solo valor de x), tengo %d: %v", len(got), got)
	}
	values := map[interface{}]bool{}
	for k, v := range got[0] {
		if k == "_id" {
			continue
		}
		values[v] = true
	}
	if !values[100] {
		t.Errorf("el valor de la primera fuente (100) no debe perderse: %v", got[0])
	}
	if !values[60] {
		t.Errorf("el valor de la segunda fuente (60) no debe perderse: %v", got[0])
	}
}

// FIX ROUND 1 / ARREGLO 3: un label igual a xKey pisaba pivoted[xKey],
// borrando el valor de x de la fila.
func TestPivotSources_LabelIgualAXKeyNoBorraLaX(t *testing.T) {
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}},
	}
	got := pivotSources(results, []string{"_id"}, "_id", "aggValue")
	if got[0]["_id"] != "Caribe" {
		t.Errorf("un label igual a xKey no debe borrar el valor de x: %v", got[0])
	}
}

// FIX ROUND 1 / ARREGLO 4: xKey lo elige el LLM; si apunta a un campo con
// un valor no comparable (un arreglo o un documento anidado de Mongo),
// usarlo directo como llave de un map[interface{}] paniquea ("hash of
// unhashable type"). Hoy eso solo lo atrapa el recover() del router, que
// lo convierte en un 500.
func TestPivotSources_XKeyNoHasheableNoPaniquea(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("pivotSources no debería paniquear con un xKey no hasheable: %v", r)
		}
	}()
	results := [][]map[string]interface{}{
		{{"_id": []interface{}{"a", "b"}, "aggValue": 10}},
		{{"_id": []interface{}{"a", "b"}, "aggValue": 20}},
	}
	got := pivotSources(results, []string{"S1", "S2"}, "_id", "aggValue")
	if len(got) != 1 {
		t.Fatalf("las dos filas con el mismo valor de x (aunque no hasheable) deben agruparse en 1, tengo %d", len(got))
	}
}

package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestResolveArtifactMonitors_AliasValidos(t *testing.T) {
	monitors := []models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	}
	aliases := buildMonitorAliases(monitors)
	sources := []models.ArtifactSource{
		{Monitor: "transacciones"},
		{Monitor: "alertas_swift"},
	}

	got, err := resolveArtifactMonitors(aliases, sources)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if len(got) != 2 || got[0].ID != monitors[0].ID || got[1].ID != monitors[1].ID {
		t.Errorf("resolución incorrecta")
	}
}

// AUTORIZACIÓN: la allowlist de un artefacto es art.MonitorIDs. Un alias
// fuera de ella no resuelve, y por lo tanto no se ejecuta ninguna
// consulta. Ver Global Constraints.
//
// Van DOS monitores a propósito: con uno solo y una sola fuente entra el
// respaldo posicional (ver TestResolveArtifactMonitors_RespaldoPosicionalTrasRenombrar),
// que resuelve al único monitor de la allowlist. Eso no debilita la
// frontera —ese monitor ESTÁ en art.MonitorIDs—, pero haría que este test
// dejara de probar lo que existe para probar. No reducir a un monitor.
func TestResolveArtifactMonitors_AliasFueraDeLaAllowlist(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	})
	sources := []models.ArtifactSource{{Monitor: "clientes_secretos"}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("un alias fuera de la allowlist debe fallar")
	}
}

// AUTORIZACIÓN: un ObjectID crudo nunca es una forma válida de nombrar un
// monitor, ni siquiera el del propio artefacto.
//
// Dos monitores por la misma razón que el test de arriba: con uno solo el
// respaldo posicional taparía la pregunta que este test hace.
func TestResolveArtifactMonitors_ObjectIDCrudoNoResuelve(t *testing.T) {
	id := primitive.NewObjectID()
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: id, Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	})
	sources := []models.ArtifactSource{{Monitor: id.Hex()}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("un ObjectID crudo no debe resolver")
	}
}

// Renombrar un monitor cambia su alias y rompería un artefacto guardado:
// el alias se deriva del NOMBRE, por eso se recalcula en cada request y
// nunca se persiste. Pero art.MonitorIDs guarda ObjectIDs ESTABLES, y un
// artefacto nace con MonitorIDs igual a los monitores a los que
// resolvieron sus propias sources. Con exactamente un monitor y una
// fuente, resolver por posición devuelve EL MISMO monitor contra el que
// se construyó el artefacto: no se amplía el acceso, se recupera el
// artefacto. Sin esto, renombrar un monitor rompe en silencio todos los
// artefactos guardados que lo referenciaban.
func TestResolveArtifactMonitors_RespaldoPosicionalTrasRenombrar(t *testing.T) {
	id := primitive.NewObjectID()
	// El monitor se llamaba "Transacciones" cuando se guardó el artefacto;
	// ahora se llama distinto, así que su alias cambió.
	aliases := buildMonitorAliases([]models.Monitor{{ID: id, Name: "Movimientos 2026"}})
	sources := []models.ArtifactSource{{Monitor: "transacciones"}}

	got, err := resolveArtifactMonitors(aliases, sources)
	if err != nil {
		t.Fatalf("con una sola fuente y un solo monitor debería resolver por posición: %v", err)
	}
	if got[0].ID != id {
		t.Error("resolvió al monitor equivocado")
	}
}

// Un chart multi-fuente pivotea, y entonces las series dejan de ser las
// YKeys del LLM para pasar a ser los labels de las fuentes. Sin esto el
// gráfico se renderiza vacío: yKeys apuntaría a "aggValue", una clave que
// el pivote ya no dejó en las filas.
func TestShape_ChartMultiFuenteDevuelveLosLabelsComoSeries(t *testing.T) {
	art := &models.ChatArtifact{
		Type: models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{
			ChartType: models.ChatChartBar,
			XKey:      "_id",
			YKeys:     []string{"aggValue"},
		},
	}
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	svc := &ArtifactService{}

	rows, series := svc.shape(art, results, []string{"Bancolombia", "Davivienda"})

	if len(series) != 2 || series[0] != "Bancolombia" || series[1] != "Davivienda" {
		t.Fatalf("series = %v, quiero los labels de las fuentes", series)
	}
	if rows[0]["Bancolombia"] != 100 || rows[0]["Davivienda"] != 60 {
		t.Errorf("la fila pivoteada no tiene una columna por fuente: %v", rows[0])
	}
	for _, key := range series {
		if _, hay := rows[0][key]; !hay {
			t.Errorf("la serie %q no existe en los datos — el gráfico saldría vacío", key)
		}
	}
}

// Con una sola fuente no hay pivote y las series siguen siendo las YKeys.
func TestShape_UnaSolaFuenteConservaLasYKeys(t *testing.T) {
	art := &models.ChatArtifact{
		Type:      models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{ChartType: models.ChatChartBar, XKey: "_id", YKeys: []string{"aggValue"}},
	}
	results := [][]map[string]interface{}{{{"_id": "Caribe", "aggValue": 100}}}
	svc := &ArtifactService{}

	_, series := svc.shape(art, results, []string{"Bancolombia"})
	if len(series) != 1 || series[0] != "aggValue" {
		t.Errorf("series = %v, quiero [aggValue]", series)
	}
}

// El respaldo posicional SOLO aplica cuando no hay ambigüedad. Con varios
// monitores, adivinar cuál era el de la fuente podría consultar el
// equivocado, y eso es peor que no mostrar nada.
func TestResolveArtifactMonitors_SinRespaldoPosicionalSiHayAmbiguedad(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Movimientos 2026"},
		{ID: primitive.NewObjectID(), Name: "Alertas 2026"},
	})
	sources := []models.ArtifactSource{{Monitor: "transacciones"}}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("con varios monitores no se debe adivinar cuál era")
	}
}

// La otra mitad de la guarda: UNA fuente. Un artefacto puede tener un solo
// monitor y varias fuentes —el mismo monitor comparado en dos ventanas de
// tiempo, la forma que describe pivotSourceLabels—, y ahí el respaldo
// tampoco aplica. No porque se elegiría el monitor equivocado (hay uno
// solo), sino porque el `continue` del respaldo apilaría ese mismo monitor
// por cada fuente irresoluble y las cuentas darían iguales: el chequeo
// len(resolved) != len(art.Sources) de RunArtifact no lo vería pasar.
// Relajar la guarda a len(aliases) == 1 rompe esto en silencio.
func TestResolveArtifactMonitors_SinRespaldoPosicionalConVariasFuentes(t *testing.T) {
	id := primitive.NewObjectID()
	aliases := buildMonitorAliases([]models.Monitor{{ID: id, Name: "Movimientos 2026"}})
	sources := []models.ArtifactSource{
		{Monitor: "movimientos_2026", Label: "2025"},
		{Monitor: "transacciones", Label: "2026"}, // alias viejo, ya no resuelve
	}

	if _, err := resolveArtifactMonitors(aliases, sources); err == nil {
		t.Fatal("con varias fuentes no hay respaldo posicional")
	}
}

// pivotSources DESAMBIGUA los labels repetidos (la segunda fuente pasa a
// llamarse "Bancolombia (2)"), así que devolver los labels crudos como
// Series apuntaría a una columna que el pivote no creó: el gráfico
// saldría con una serie vacía. Series tiene que ser SIEMPRE el nombre
// real de la columna pivoteada.
func TestShape_ChartMultiFuenteConLabelsRepetidosUsaLosNombresDesambiguados(t *testing.T) {
	art := &models.ChatArtifact{
		Type: models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{
			ChartType: models.ChatChartBar,
			XKey:      "_id",
			YKeys:     []string{"aggValue"},
		},
	}
	// Mismo monitor comparado en dos ventanas de tiempo: mismo label.
	results := [][]map[string]interface{}{
		{{"_id": "Caribe", "aggValue": 100}},
		{{"_id": "Caribe", "aggValue": 60}},
	}
	svc := &ArtifactService{}

	rows, series := svc.shape(art, results, []string{"Bancolombia", "Bancolombia"})

	if len(series) != 2 || series[0] == series[1] {
		t.Fatalf("series = %v, las dos series no pueden llamarse igual", series)
	}
	for _, key := range series {
		if _, hay := rows[0][key]; !hay {
			t.Errorf("la serie %q no existe en los datos %v — el gráfico saldría vacío", key, rows[0])
		}
	}
}

// Una TABLA con varias fuentes concatena (con su columna discriminadora)
// en vez de pivotear, y sus series siguen siendo las YKeys.
func TestShape_TablaMultiFuenteConcatena(t *testing.T) {
	art := &models.ChatArtifact{Type: models.ChatArtifactTable}
	results := [][]map[string]interface{}{
		{{"monto": 100}},
		{{"monto": 60}},
	}
	svc := &ArtifactService{}

	rows, series := svc.shape(art, results, []string{"Bancolombia", "Davivienda"})
	if len(rows) != 2 {
		t.Fatalf("una tabla multi-fuente debe apilar las filas: %v", rows)
	}
	if rows[0][artifactMonitorColumn] != "Bancolombia" || rows[1][artifactMonitorColumn] != "Davivienda" {
		t.Errorf("falta la columna discriminadora: %v", rows)
	}
	if len(series) != 0 {
		t.Errorf("una tabla sin chartSpec no tiene series: %v", series)
	}
}

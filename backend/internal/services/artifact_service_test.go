package services

import (
	"context"
	"errors"
	"strings"
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

	got, err := resolveArtifactMonitors(aliases, sources, 2)
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

	if _, err := resolveArtifactMonitors(aliases, sources, 2); err == nil {
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

	if _, err := resolveArtifactMonitors(aliases, sources, 2); err == nil {
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

	got, err := resolveArtifactMonitors(aliases, sources, 1)
	if err != nil {
		t.Fatalf("con una sola fuente y un solo monitor debería resolver por posición: %v", err)
	}
	if got[0].ID != id {
		t.Error("resolvió al monitor equivocado")
	}
}

// ARREGLO 2: la precondición del respaldo es len(art.MonitorIDs) == 1, no
// len(aliases) == 1. `aliases` excluye los monitores BORRADOS, así que una
// allowlist de dos monitores con uno borrado deja un solo alias — y sin
// este chequeo una fuente que nombraba al borrado se ejecutaría contra el
// sobreviviente en silencio, que es justo el "nunca se ejecuta contra un
// monitor distinto del que se resolvió" que el spec prohíbe.
func TestResolveArtifactMonitors_SinRespaldoSiLaAllowlistTeniaOtroMonitor(t *testing.T) {
	// art.MonitorIDs = [A, B]; A fue borrado, así que solo B llega acá.
	b := primitive.NewObjectID()
	aliases := buildMonitorAliases([]models.Monitor{{ID: b, Name: "Alertas SWIFT"}})
	sources := []models.ArtifactSource{{Monitor: "transacciones"}} // nombraba a A

	if _, err := resolveArtifactMonitors(aliases, sources, 2); err == nil {
		t.Fatal("no se debe caer al sobreviviente cuando la allowlist tenía otro monitor")
	}
}

// ARREGLO 3: el handler tiene que poder distinguir "la fuente ya no está"
// de "Mongo falló un segundo" sin comparar cadenas. La primera habilita el
// modo degradado del spec (caché + aviso); la segunda NO puede mostrar
// datos viejos como si estuvieran vigentes.
func TestResolveArtifactMonitors_ErrorEsSourceUnavailable(t *testing.T) {
	aliases := buildMonitorAliases([]models.Monitor{
		{ID: primitive.NewObjectID(), Name: "Transacciones"},
		{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"},
	})
	sources := []models.ArtifactSource{{Monitor: "clientes_secretos"}}

	_, err := resolveArtifactMonitors(aliases, sources, 2)
	if !errors.Is(err, ErrArtifactSourceUnavailable) {
		t.Fatalf("err = %v, quiero que envuelva ErrArtifactSourceUnavailable", err)
	}
	// El texto original de resolveQueryMonitor no se pierde: sigue siendo
	// lo que se diagnostica en los logs.
	if !strings.Contains(err.Error(), "clientes_secretos") {
		t.Errorf("el error perdió el alias que falló: %v", err)
	}
}

// El monitor único del artefacto fue borrado: allowlistSize == 1 pero
// aliases queda VACÍO (buildMonitorAliases excluye a los borrados). El
// respaldo posicional exige aliases == 1 además de allowlistSize == 1 —
// sin ese conjuntivo, el `for range aliases` no itera, el respaldo agrega
// cero monitores y devuelve nil sin error, y RunArtifact reporta un error
// llano (por el chequeo de longitud) sin envolver
// ErrArtifactSourceUnavailable: el handler ya no puede activar el modo
// degradado (caché + aviso) y el punto ciego queda reabierto en silencio.
func TestResolveArtifactMonitors_SoloMonitorBorradoEsSourceUnavailable(t *testing.T) {
	aliases := map[string]*models.Monitor{} // el único monitor del artefacto ya no existe
	sources := []models.ArtifactSource{{Monitor: "transacciones"}}

	_, err := resolveArtifactMonitors(aliases, sources, 1)
	if !errors.Is(err, ErrArtifactSourceUnavailable) {
		t.Fatalf("err = %v, quiero que envuelva ErrArtifactSourceUnavailable", err)
	}
}

// ARREGLO 4: con UN solo monitor en la allowlist, el chequeo de alias es un
// NO-OP — cualquier alias resuelve, incluso el Hex() de otro monitor. Es
// una DECISIÓN, no un descuido, y por eso está fijada acá: el respaldo
// posicional no puede distinguir un nombre viejo de uno inventado, y no
// necesita hacerlo, porque el resultado está acotado al único monitor que
// el artefacto ya tenía autorizado. La frontera no la sostiene el alias:
// la sostiene que `aliases` se construya solo con art.MonitorIDs.
//
// Los tests de allowlist de arriba usan DOS monitores justamente porque
// con uno no habría nada que probar.
func TestResolveArtifactMonitors_ConUnSoloMonitorElAliasEsIrrelevante(t *testing.T) {
	unico := primitive.NewObjectID()
	otro := primitive.NewObjectID() // no está en la allowlist del artefacto
	aliases := buildMonitorAliases([]models.Monitor{{ID: unico, Name: "Transacciones"}})
	sources := []models.ArtifactSource{{Monitor: otro.Hex()}}

	got, err := resolveArtifactMonitors(aliases, sources, 1)
	if err != nil {
		t.Fatalf("con un solo monitor el respaldo resuelve igual: %v", err)
	}
	if len(got) != 1 || got[0].ID != unico {
		t.Fatalf("el resultado debe estar acotado al monitor del artefacto, got = %v", got)
	}
}

// ARREGLO 1: art.UserID != userID ⇒ ErrArtifactNotFound, indistinguible de
// "no existe". Sin este test, borrar el chequeo no rompe nada.
//
// Sources NO vacío a propósito: si estuviera vacío el test pasaría por la
// razón equivocada (ErrArtifactNotRerunnable), sin llegar a probar la
// propiedad. Los repos van en nil porque ambos guardas retornan ANTES de
// tocar el repositorio — que no haya panic es parte de lo que se afirma.
func TestRunArtifact_DeOtroUsuarioEsNotFound(t *testing.T) {
	art := &models.ChatArtifact{
		UserID:  primitive.NewObjectID(),
		Type:    models.ChatArtifactTable,
		Sources: []models.ArtifactSource{{Monitor: "transacciones"}},
	}
	svc := &ArtifactService{}

	_, err := svc.RunArtifact(context.Background(), art, primitive.NewObjectID())
	if !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("err = %v, quiero ErrArtifactNotFound", err)
	}
}

// ARREGLO 1: sources vacío ⇒ instantánea, NUNCA una ejecución.
func TestRunArtifact_SinSourcesNoEjecuta(t *testing.T) {
	userID := primitive.NewObjectID()
	art := &models.ChatArtifact{
		UserID:    userID,
		Type:      models.ChatArtifactChart,
		ChartSpec: &models.ChartSpec{ChartType: models.ChatChartBar, Data: []map[string]interface{}{{"x": 1}}},
	}
	svc := &ArtifactService{}

	_, err := svc.RunArtifact(context.Background(), art, userID)
	if !errors.Is(err, ErrArtifactNotRerunnable) {
		t.Fatalf("err = %v, quiero ErrArtifactNotRerunnable", err)
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

	if _, err := resolveArtifactMonitors(aliases, sources, 2); err == nil {
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

	if _, err := resolveArtifactMonitors(aliases, sources, 1); err == nil {
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

// ---------------------------------------------------------------------------
// HALLAZGO 1 (revisión de rama): una fuente re-ejecutándose contra OTRO
// monitor.
//
// El alias se deriva del nombre y se TRUNCA en 40 caracteres
// (monitorAliasMaxLen), así que dos monitores descriptivos que solo
// difieren en el año colisionan: los dos dan
// "transacciones_internacionales_bancolombi" y buildMonitorAliases le
// cuelga "_2" al SEGUNDO de la lista. Cuál es el segundo depende del
// orden, y el orden cambia entre la conversación (conv.MonitorIDs) y el
// artefacto (art.MonitorIDs, derivado de las sources, que el LLM pudo
// listar al revés). Con el orden invertido, la fuente 0 se ejecutaba
// contra el monitor de la fuente 1 llevando puesto el rótulo de la 0:
// serie mal etiquetada, columna _monitor mentirosa, cero errores.
//
// El fixture es compartido por los dos tests de abajo, que son un PAR: uno
// prueba el arreglo, el otro deja constancia empírica del bug en el camino
// viejo (que sigue vivo, sin migración, para los artefactos ya guardados).
func colisionDeAliasFixture() (monitor2024, monitor2025 models.Monitor) {
	return models.Monitor{ID: primitive.NewObjectID(), Name: "Transacciones internacionales Bancolombia 2024"},
		models.Monitor{ID: primitive.NewObjectID(), Name: "Transacciones internacionales Bancolombia 2025"}
}

func TestBindArtifactSources_LaFuenteQuedaAtadaAlMonitorDeSuTurno(t *testing.T) {
	m2024, m2025 := colisionDeAliasFixture()
	if monitorAlias(m2024.Name) != monitorAlias(m2025.Name) {
		t.Fatalf("el fixture ya no colisiona (%q vs %q): el test dejó de probar lo que existe para probar",
			monitorAlias(m2024.Name), monitorAlias(m2025.Name))
	}

	// La conversación: [2024, 2025]. El alias base queda para el 2024 y el
	// 2025 se lleva el "_2".
	convAliases := buildMonitorAliases([]models.Monitor{m2024, m2025})
	base := monitorAlias(m2024.Name)

	// El artefacto que escribió el LLM lista PRIMERO el 2025.
	art := &models.ChatArtifact{
		Type: models.ChatArtifactTable,
		Sources: []models.ArtifactSource{
			{Monitor: base + "_2", Label: "2025"},
			{Monitor: base, Label: "2024"},
		},
	}
	art.MonitorIDs = bindArtifactSources(convAliases, art.Sources)

	if len(art.MonitorIDs) != 2 || art.MonitorIDs[0] != m2025.ID || art.MonitorIDs[1] != m2024.ID {
		t.Fatalf("MonitorIDs = %v; el orden lo dan las sources: [2025, 2024]", art.MonitorIDs)
	}
	if art.Sources[0].MonitorID != m2025.ID || art.Sources[1].MonitorID != m2024.ID {
		t.Fatalf("el vínculo quedó mal estampado: %s / %s",
			art.Sources[0].MonitorID.Hex(), art.Sources[1].MonitorID.Hex())
	}

	// Re-ejecución: los alias se reconstruyen desde art.MonitorIDs, o sea
	// [2025, 2024] — invertido respecto de la conversación. Acá el alias
	// base ya es el del 2025 y el "_2" el del 2024: si la resolución fuera
	// por alias, cada fuente consultaría el monitor de la otra.
	runAliases := buildMonitorAliases([]models.Monitor{m2025, m2024})
	resolved, err := resolveArtifactMonitors(runAliases, art.Sources, len(art.MonitorIDs))
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if resolved[0].ID != m2025.ID {
		t.Errorf("la fuente rotulada 2025 se ejecutó contra %q", resolved[0].Name)
	}
	if resolved[1].ID != m2024.ID {
		t.Errorf("la fuente rotulada 2024 se ejecutó contra %q", resolved[1].Name)
	}
}

// La otra mitad del par: SIN el vínculo estampado —un artefacto guardado
// antes de este arreglo— la resolución sigue siendo por alias y por lo
// tanto sigue invirtiéndose. Está acá para que quede a la vista que el
// arreglo es el id y no otra cosa, y para fijar que el camino viejo no
// cambió: es el que usan los artefactos ya persistidos, que no se migran.
func TestResolveArtifactMonitors_SinVinculoElAliasSigueSiendoElCamino(t *testing.T) {
	m2024, m2025 := colisionDeAliasFixture()
	base := monitorAlias(m2024.Name)
	sources := []models.ArtifactSource{
		{Monitor: base + "_2", Label: "2025"}, // sin MonitorID: artefacto viejo
		{Monitor: base, Label: "2024"},
	}

	runAliases := buildMonitorAliases([]models.Monitor{m2025, m2024})
	resolved, err := resolveArtifactMonitors(runAliases, sources, 2)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	// El alias manda, y con este orden "base_2" es el 2024: exactamente el
	// comportamiento anterior, intacto.
	if resolved[0].ID != m2024.ID || resolved[1].ID != m2025.ID {
		t.Fatalf("el camino por alias cambió de comportamiento: %q / %q", resolved[0].Name, resolved[1].Name)
	}
}

// AUTORIZACIÓN. El vínculo es un REGISTRO de a qué monitor se ató la
// fuente, jamás una llave para salir de la allowlist: un id que no está
// entre los monitores vivos de art.MonitorIDs no resuelve y no ejecuta
// ninguna consulta. Acá los alias se construyen solo con B y la fuente
// viene estampada con A.
func TestResolveArtifactMonitors_VinculoFueraDeLaAllowlistNoResuelve(t *testing.T) {
	monitorA := primitive.NewObjectID()
	monitorB := models.Monitor{ID: primitive.NewObjectID(), Name: "Alertas SWIFT"}
	aliases := buildMonitorAliases([]models.Monitor{monitorB})
	sources := []models.ArtifactSource{{Monitor: "alertas_swift", MonitorID: monitorA}}

	got, err := resolveArtifactMonitors(aliases, sources, 1)
	if err == nil {
		t.Fatalf("un vínculo fuera de la allowlist no puede resolver; devolvió %v", got)
	}
	if !errors.Is(err, ErrArtifactSourceUnavailable) {
		t.Fatalf("err = %v, quiero ErrArtifactSourceUnavailable (modo degradado: caché con aviso)", err)
	}
	// Y NO cae al camino por alias: ahí "alertas_swift" resolvería a B y la
	// fuente de A se ejecutaría contra B en silencio, que es justo lo que
	// el spec prohíbe ("nunca se ejecuta contra un monitor distinto del que
	// se resolvió").
	if got != nil {
		t.Fatalf("no debe resolver a ningún monitor, resolvió %v", got)
	}
}

// El vínculo tampoco se deja arrastrar por el respaldo posicional: con un
// solo monitor en la allowlist, una fuente estampada contra un monitor
// BORRADO falla en vez de caer sobre el que quedó. El respaldo posicional
// existe para artefactos sin vínculo (renombrados), no para pasarle por
// encima a uno que sí sabe contra qué se creó.
func TestResolveArtifactMonitors_VinculoABorradoNoUsaElRespaldoPosicional(t *testing.T) {
	borrado := primitive.NewObjectID()
	sobreviviente := models.Monitor{ID: primitive.NewObjectID(), Name: "Transacciones"}
	aliases := buildMonitorAliases([]models.Monitor{sobreviviente})
	sources := []models.ArtifactSource{{Monitor: "transacciones_viejas", MonitorID: borrado}}

	if _, err := resolveArtifactMonitors(aliases, sources, 1); !errors.Is(err, ErrArtifactSourceUnavailable) {
		t.Fatalf("err = %v, quiero ErrArtifactSourceUnavailable", err)
	}
}

// Un alias que no resuelve al crear no puede fabricar acceso: se omite de
// la allowlist y queda sin vínculo, así que al re-ejecutar recorre el
// camino por alias como cualquier artefacto viejo. Sin esto, empezar a
// estampar podría haber roto la creación de artefactos con una fuente
// inventada por el LLM, que hoy simplemente se ignora.
func TestBindArtifactSources_AliasDesconocidoNoEstampaNiEntraALaAllowlist(t *testing.T) {
	monitor := models.Monitor{ID: primitive.NewObjectID(), Name: "Transacciones"}
	aliases := buildMonitorAliases([]models.Monitor{monitor})
	sources := []models.ArtifactSource{
		{Monitor: "transacciones"},
		{Monitor: "clientes_secretos"},
	}

	ids := bindArtifactSources(aliases, sources)
	if len(ids) != 1 || ids[0] != monitor.ID {
		t.Fatalf("MonitorIDs = %v, quiero solo el monitor resuelto", ids)
	}
	if sources[0].MonitorID != monitor.ID {
		t.Error("la fuente resoluble debe quedar atada")
	}
	if !sources[1].MonitorID.IsZero() {
		t.Errorf("la fuente irresoluble no puede quedar atada a nada: %s", sources[1].MonitorID.Hex())
	}
}

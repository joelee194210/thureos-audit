package models

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestNormalize_DocumentoLegacySeConvierteAMonitorIDs(t *testing.T) {
	id := primitive.NewObjectID()
	conv := ChatConversation{LegacyMonitorID: &id}
	conv.Normalize()

	if len(conv.MonitorIDs) != 1 || conv.MonitorIDs[0] != id {
		t.Fatalf("quiero MonitorIDs=[%s], tengo %v", id.Hex(), conv.MonitorIDs)
	}
	if conv.LegacyMonitorID != nil {
		t.Error("LegacyMonitorID debe quedar en nil después de normalizar")
	}
}

func TestNormalize_DocumentoNuevoNoSeToca(t *testing.T) {
	ids := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()}
	conv := ChatConversation{MonitorIDs: ids}
	conv.Normalize()

	if len(conv.MonitorIDs) != 2 || conv.MonitorIDs[0] != ids[0] || conv.MonitorIDs[1] != ids[1] {
		t.Fatalf("MonitorIDs no debe cambiar; tengo %v", conv.MonitorIDs)
	}
}

// Un documento a medio migrar (ambos campos presentes) debe quedarse con
// monitor_ids: es el campo que se escribe hoy, el legacy es residuo.
func TestNormalize_AmbosCamposGanaMonitorIDs(t *testing.T) {
	nuevo := primitive.NewObjectID()
	viejo := primitive.NewObjectID()
	conv := ChatConversation{MonitorIDs: []primitive.ObjectID{nuevo}, LegacyMonitorID: &viejo}
	conv.Normalize()

	if len(conv.MonitorIDs) != 1 || conv.MonitorIDs[0] != nuevo {
		t.Fatalf("quiero que gane monitor_ids (%s), tengo %v", nuevo.Hex(), conv.MonitorIDs)
	}
}

func TestNormalize_SinNingunCampoQuedaVacio(t *testing.T) {
	conv := ChatConversation{}
	conv.Normalize()
	if len(conv.MonitorIDs) != 0 {
		t.Fatalf("quiero MonitorIDs vacío, tengo %v", conv.MonitorIDs)
	}
}

// El defecto que este test fija: una conversación legacy (monitor_id
// escalar, sin monitor_ids) perdía su monitor original al agregarle uno
// nuevo. El $addToSet anterior creaba monitor_ids=[nuevo] y dejaba el
// escalar intacto; en la lectura siguiente Normalize veía len(MonitorIDs)
// == 1 y ya no miraba el campo legacy, así que el monitor original
// desaparecía de la allowlist. Acá se fija la secuencia completa
// Normalize → AppendMonitorID, que es lo que el repositorio persiste.
func TestAppendMonitorID_ConversacionLegacyConservaSuMonitorOriginal(t *testing.T) {
	original := primitive.NewObjectID()
	nuevo := primitive.NewObjectID()

	conv := ChatConversation{LegacyMonitorID: &original}
	conv.Normalize()

	next, changed := AppendMonitorID(conv.MonitorIDs, nuevo)
	if !changed {
		t.Fatal("agregar un monitor ausente debe reportar changed=true")
	}
	if len(next) != 2 {
		t.Fatalf("quiero los dos monitores, tengo %v", next)
	}
	if next[0] != original {
		t.Errorf("el monitor legacy debe quedar PRIMERO (buildMonitorAliases usa el orden); tengo %v", next)
	}
	if next[1] != nuevo {
		t.Errorf("el monitor nuevo debe quedar al final; tengo %v", next)
	}

	// Y lo persistido debe sobrevivir a la normalización siguiente sin que
	// el campo legacy (ya borrado por el repositorio) haga falta.
	releida := ChatConversation{MonitorIDs: next}
	releida.Normalize()
	if len(releida.MonitorIDs) != 2 || releida.MonitorIDs[0] != original {
		t.Errorf("releer lo persistido debe devolver los dos monitores en orden; tengo %v", releida.MonitorIDs)
	}
}

func TestAppendMonitorID_MonitorYaPresenteEsNoOp(t *testing.T) {
	a := primitive.NewObjectID()
	b := primitive.NewObjectID()
	existing := []primitive.ObjectID{a, b}

	next, changed := AppendMonitorID(existing, a)
	if changed {
		t.Error("agregar un monitor ya presente no debe reportar cambio")
	}
	if len(next) != 2 || next[0] != a || next[1] != b {
		t.Errorf("el conjunto no debe cambiar; tengo %v", next)
	}
}

func TestAppendMonitorID_NoMutaElSliceDeEntrada(t *testing.T) {
	a := primitive.NewObjectID()
	existing := make([]primitive.ObjectID, 1, 4) // capacidad de sobra: un append ingenuo escribiría en el mismo array
	existing[0] = a
	b := primitive.NewObjectID()

	next, _ := AppendMonitorID(existing, b)
	if len(existing) != 1 || existing[0] != a {
		t.Errorf("el slice de entrada no debe mutarse; tengo %v", existing)
	}
	if len(next) != 2 || next[1] != b {
		t.Errorf("quiero [a b]; tengo %v", next)
	}
}

func TestAppendMonitorID_ConjuntoVacioQuedaConUnSoloMonitor(t *testing.T) {
	id := primitive.NewObjectID()
	next, changed := AppendMonitorID(nil, id)
	if !changed || len(next) != 1 || next[0] != id {
		t.Fatalf("quiero [%s] con changed=true; tengo %v (changed=%v)", id.Hex(), next, changed)
	}
}

func TestIsRerunnable_ConSourcesSi(t *testing.T) {
	art := ChatArtifact{
		Type:    ChatArtifactTable,
		Sources: []ArtifactSource{{Monitor: "transacciones"}},
	}
	if !art.IsRerunnable() {
		t.Error("un artefacto con sources debe ser re-ejecutable")
	}
}

// La regla del spec es una sola: sources vacío ⇒ instantánea. Cubre por
// igual a los artefactos legacy y a los custom.
func TestIsRerunnable_SinSourcesNo(t *testing.T) {
	legacy := ChatArtifact{
		Type:      ChatArtifactTable,
		ChartSpec: &ChartSpec{Data: []map[string]interface{}{{"a": 1}}},
	}
	if legacy.IsRerunnable() {
		t.Error("un artefacto sin sources es instantánea, no re-ejecutable")
	}

	custom := ChatArtifact{Type: ChatArtifactCustom, Code: "<p>hola</p>"}
	if custom.IsRerunnable() {
		t.Error("un artefacto custom nunca es re-ejecutable")
	}
}

func TestFromLegacyArtifact_ConservaLosDatosComoInstantanea(t *testing.T) {
	legacy := &LegacyChatArtifact{
		Type:      ChatArtifactTable,
		Title:     "Ventas",
		ChartSpec: &ChartSpec{Data: []map[string]interface{}{{"region": "Caribe"}}},
	}
	art := FromLegacyArtifact(legacy)

	if art.Title != "Ventas" || art.Type != ChatArtifactTable {
		t.Errorf("no conservó tipo/título: %+v", art)
	}
	if len(art.ChartSpec.Data) != 1 {
		t.Error("debe conservar los datos guardados")
	}
	if art.IsRerunnable() {
		t.Error("un artefacto legacy nunca es re-ejecutable")
	}
	if !art.ID.IsZero() {
		t.Error("un artefacto legacy no tiene id propio")
	}
}

func TestFromLegacyArtifact_NilDevuelveNil(t *testing.T) {
	if FromLegacyArtifact(nil) != nil {
		t.Error("nil debe devolver nil")
	}
}

// HALLAZGO 2 (revisión de rama): un artefacto legacy se serializaba con
// "id":"000000000000000000000000". Esa cadena es TRUTHY en JavaScript, así
// que los guardas del frontend (`artifact.id ?? artifact.title`,
// `disabled={!artifact.id}`) no se disparaban: "Guardar" quedaba habilitado
// y devolvía 404, y todos los artefactos legacy compartían key de React,
// por lo que el canvas no se remontaba al cambiar de conversación.
// `json:"id,omitempty"` no lo arregla —un [12]byte nunca es "empty" para
// encoding/json—, de ahí el marshaler propio.
func TestChatArtifactMarshalJSON_LegacySinIdNoSerializaElIdCero(t *testing.T) {
	art := FromLegacyArtifact(&LegacyChatArtifact{
		Type:      ChatArtifactTable,
		Title:     "Ventas",
		ChartSpec: &ChartSpec{Data: []map[string]interface{}{{"region": "Caribe"}}},
	})

	raw, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("el JSON no es un objeto: %v", err)
	}

	for _, key := range []string{"id", "messageId", "ranAt"} {
		if _, present := decoded[key]; present {
			t.Errorf("%q no debe estar en el JSON de un artefacto legacy: %s", key, raw)
		}
	}
	// El resto del artefacto sigue viajando: el marshaler omite, no recorta.
	if decoded["title"] != "Ventas" {
		t.Errorf("el título se perdió: %s", raw)
	}
	if decoded["type"] != string(ChatArtifactTable) {
		t.Errorf("el tipo se perdió: %s", raw)
	}
	if _, ok := decoded["chartSpec"]; !ok {
		t.Errorf("el chartSpec se perdió: %s", raw)
	}
}

// La contracara: un artefacto real sí lleva sus tres campos, con el mismo
// valor de siempre. Sin este test, "omitir el cero" podría degenerar en
// "omitir siempre" y nadie se enteraría.
func TestChatArtifactMarshalJSON_ArtefactoRealConservaSusCampos(t *testing.T) {
	id := primitive.NewObjectID()
	msgID := primitive.NewObjectID()
	ranAt := time.Date(2026, 3, 15, 10, 0, 0, 0, time.UTC)
	art := ChatArtifact{
		ID:        id,
		MessageID: msgID,
		RanAt:     ranAt,
		Type:      ChatArtifactTable,
		Title:     "Ventas",
	}

	raw, err := json.Marshal(art)
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("el JSON no es un objeto: %v", err)
	}

	if decoded["id"] != id.Hex() {
		t.Errorf("id = %v, quiero %s", decoded["id"], id.Hex())
	}
	if decoded["messageId"] != msgID.Hex() {
		t.Errorf("messageId = %v, quiero %s", decoded["messageId"], msgID.Hex())
	}
	if decoded["ranAt"] != "2026-03-15T10:00:00Z" {
		t.Errorf("ranAt = %v, quiero la fecha de corrida", decoded["ranAt"])
	}
}

// El marshaler también tiene que funcionar cuando el artefacto viaja
// adentro de otra cosa: el chat devuelve el mensaje completo, no el
// artefacto suelto, y un método con receptor por valor cubre los dos.
func TestChatArtifactMarshalJSON_DentroDeUnMensajeYDeUnaLista(t *testing.T) {
	legacy := FromLegacyArtifact(&LegacyChatArtifact{Type: ChatArtifactTable, Title: "Ventas"})

	msg, err := json.Marshal(ChatMessage{Role: ChatRoleAssistant, Artifact: legacy})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	var decodedMsg struct {
		Artifact map[string]interface{} `json:"artifact"`
	}
	if err := json.Unmarshal(msg, &decodedMsg); err != nil {
		t.Fatalf("el JSON no es un objeto: %v", err)
	}
	if _, present := decodedMsg.Artifact["id"]; present {
		t.Errorf("el id cero se coló en el artefacto del mensaje: %s", msg)
	}

	list, err := json.Marshal([]ChatArtifact{*legacy})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	var decodedList []map[string]interface{}
	if err := json.Unmarshal(list, &decodedList); err != nil {
		t.Fatalf("el JSON no es una lista de objetos: %v", err)
	}
	if _, present := decodedList[0]["id"]; present {
		t.Errorf("el id cero se coló en la lista (biblioteca): %s", list)
	}
}

// HALLAZGO 1 (revisión de rama): ArtifactSource.MonitorID tiene que ser
// INVISIBLE para JSON. llmArtifactInput.Sources es []ArtifactSource, así
// que un tag json acá lo volvería escribible por el LLM —cuyo texto está
// moldeado por los CSV que suben los usuarios— y sería elegir el monitor a
// consultar, el mismo agujero que el DTO angosto cerró.
func TestArtifactSource_MonitorIDNoSeDeserializaDesdeJSON(t *testing.T) {
	intruso := primitive.NewObjectID()
	raw := `{"monitor":"transacciones","label":"Bancolombia",
	         "monitorId":"` + intruso.Hex() + `",
	         "monitor_id":"` + intruso.Hex() + `",
	         "MonitorID":"` + intruso.Hex() + `"}`

	var src ArtifactSource
	if err := json.Unmarshal([]byte(raw), &src); err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	if !src.MonitorID.IsZero() {
		t.Fatalf("MonitorID = %s; ninguna forma del nombre puede llegar desde JSON", src.MonitorID.Hex())
	}
	if src.Monitor != "transacciones" {
		t.Errorf("los campos legítimos sí se deserializan: %+v", src)
	}
}

func TestArtifactSource_MonitorIDNoSeSerializaAJSON(t *testing.T) {
	raw, err := json.Marshal(ArtifactSource{Monitor: "transacciones", MonitorID: primitive.NewObjectID()})
	if err != nil {
		t.Fatalf("no esperaba error: %v", err)
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("el JSON no es un objeto: %v", err)
	}
	for key := range decoded {
		if strings.Contains(strings.ToLower(key), "monitorid") || key == "monitor_id" {
			t.Errorf("el id del monitor no debe salir al cliente: %s", raw)
		}
	}
}

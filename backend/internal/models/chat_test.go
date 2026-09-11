package models

import (
	"testing"

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

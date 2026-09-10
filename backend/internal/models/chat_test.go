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

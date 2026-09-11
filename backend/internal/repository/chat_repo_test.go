package repository

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// ramas devuelve el $or del filtro, que es donde vive toda la lógica.
func ramas(t *testing.T, filter bson.M) []bson.M {
	t.Helper()
	or, ok := filter["$or"].([]bson.M)
	if !ok {
		t.Fatalf("el filtro debe traer un $or de bson.M; tengo %#v", filter["$or"])
	}
	return or
}

// El caso confirmado del hallazgo: una conversación legacy no tiene
// monitor_ids, guarda su único monitor en el escalar monitor_id. El
// compare-and-set tiene que poder matchearla o la escritura que rescata
// ese monitor nunca se aplica. {"monitor_ids": nil} matchea campo ausente
// además de nulo — verificado contra mongo:7.
func TestMonitorSetFilter_DocumentoLegacyMatcheaPorElEscalar(t *testing.T) {
	convID := primitive.NewObjectID()
	original := primitive.NewObjectID()

	filter := monitorSetFilter(convID, []primitive.ObjectID{original})

	if filter["_id"] != convID {
		t.Errorf("el filtro debe acotar por _id; tengo %v", filter["_id"])
	}
	or := ramas(t, filter)
	if len(or) != 2 {
		t.Fatalf("con un solo monitor observado tiene que haber rama nueva y rama legacy; tengo %d", len(or))
	}
	legacy := or[1]
	if _, presente := legacy["monitor_ids"]; !presente {
		t.Error("la rama legacy debe exigir monitor_ids ausente o nulo")
	}
	if legacy["monitor_ids"] != nil {
		t.Errorf("la rama legacy debe comparar monitor_ids contra nil; tengo %#v", legacy["monitor_ids"])
	}
	if legacy["monitor_id"] != original {
		t.Errorf("la rama legacy debe exigir el escalar monitor_id == %s; tengo %v", original.Hex(), legacy["monitor_id"])
	}
}

// La rama "nueva" compara el array completo y ordenado: si alguien más
// agregó un monitor en el medio, no matchea y la escritura no pisa nada.
func TestMonitorSetFilter_DocumentoNuevoComparaElArrayCompleto(t *testing.T) {
	convID := primitive.NewObjectID()
	observed := []primitive.ObjectID{primitive.NewObjectID(), primitive.NewObjectID()}

	or := ramas(t, monitorSetFilter(convID, observed))
	if len(or) != 1 {
		t.Fatalf("con más de un monitor observado la forma legacy es imposible: sobra la segunda rama; tengo %d", len(or))
	}
	got, ok := or[0]["monitor_ids"].([]primitive.ObjectID)
	if !ok {
		t.Fatalf("la rama debe comparar monitor_ids contra el array observado; tengo %#v", or[0]["monitor_ids"])
	}
	if len(got) != 2 || got[0] != observed[0] || got[1] != observed[1] {
		t.Errorf("el array comparado debe ser el observado, en orden; tengo %v", got)
	}
}

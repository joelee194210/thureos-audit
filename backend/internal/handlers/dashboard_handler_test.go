package handlers

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Antes de este cambio, Get/GetData/DrillDown no llamaban a este chequeo en
// absoluto: cualquier usuario autenticado que conociera o adivinara un ID de
// dashboard podía leer los datos completos de un dashboard privado ajeno.
func TestCanReadDashboard_ViewerSinRelacionQuedaFuera(t *testing.T) {
	owner := primitive.NewObjectID()
	stranger := primitive.NewObjectID()
	d := &models.Dashboard{OwnerID: owner, IsPublic: false}

	if canReadDashboard(d, stranger, string(models.RoleViewer)) {
		t.Error("un viewer sin relación con el dashboard no debería poder leerlo")
	}
}

func TestCanReadDashboard_OrgRolesSiemprePasan(t *testing.T) {
	owner := primitive.NewObjectID()
	other := primitive.NewObjectID()
	d := &models.Dashboard{OwnerID: owner, IsPublic: false}

	if !canReadDashboard(d, other, string(models.RoleAdmin)) {
		t.Error("un admin debe poder leer cualquier dashboard, sea o no el dueño")
	}
	if !canReadDashboard(d, other, string(models.RoleCompliance)) {
		t.Error("compliance ve la plataforma completa, según CLAUDE.md: debe poder leer el dashboard")
	}
}

func TestCanReadDashboard_DuenoYCompartidoPasan(t *testing.T) {
	owner := primitive.NewObjectID()
	sharedUser := primitive.NewObjectID()
	stranger := primitive.NewObjectID()
	d := &models.Dashboard{OwnerID: owner, SharedWith: []primitive.ObjectID{sharedUser}}

	if !canReadDashboard(d, owner, string(models.RoleViewer)) {
		t.Error("el dueño debe poder leer su propio dashboard")
	}
	if !canReadDashboard(d, sharedUser, string(models.RoleViewer)) {
		t.Error("un usuario en SharedWith debe poder leer el dashboard")
	}
	if canReadDashboard(d, stranger, string(models.RoleViewer)) {
		t.Error("un usuario fuera de SharedWith no debe poder leer el dashboard")
	}
}

func TestCanReadDashboard_PublicoLeeCualquiera(t *testing.T) {
	d := &models.Dashboard{OwnerID: primitive.NewObjectID(), IsPublic: true}
	if !canReadDashboard(d, primitive.NewObjectID(), string(models.RoleViewer)) {
		t.Error("un dashboard público debe ser legible por cualquier usuario autenticado")
	}
}

// Un dashboard público es legible por cualquiera, pero no editable por
// cualquiera: IsPublic no debe colarse en la condición de escritura.
func TestCanWriteDashboard_PublicoNoEsEditablePorCualquiera(t *testing.T) {
	d := &models.Dashboard{OwnerID: primitive.NewObjectID(), IsPublic: true}
	if canWriteDashboard(d, primitive.NewObjectID(), string(models.RoleViewer)) {
		t.Error("que un dashboard sea público no debe habilitar a cualquiera a modificarlo")
	}
}

func TestCanWriteDashboard_DuenoYCompartidoPasanOrgRolesTambien(t *testing.T) {
	owner := primitive.NewObjectID()
	sharedUser := primitive.NewObjectID()
	stranger := primitive.NewObjectID()
	d := &models.Dashboard{OwnerID: owner, SharedWith: []primitive.ObjectID{sharedUser}}

	if !canWriteDashboard(d, owner, string(models.RoleViewer)) {
		t.Error("el dueño debe poder modificar su propio dashboard")
	}
	if !canWriteDashboard(d, sharedUser, string(models.RoleViewer)) {
		t.Error("un usuario en SharedWith debe poder modificar el dashboard")
	}
	if canWriteDashboard(d, stranger, string(models.RoleViewer)) {
		t.Error("un usuario ajeno al dashboard no debe poder modificarlo")
	}
	if !canWriteDashboard(d, stranger, string(models.RoleAdmin)) {
		t.Error("un admin debe poder modificar cualquier dashboard")
	}
}

// Antes de este cambio, un widget sum/avg/min/max sin campo se creaba igual
// (201) y buildAggExpression armaba un field path vacío ("$"), que Mongo
// recién rechazaba al consultar los datos — el widget quedaba roto y solo
// se descubría al verlo.
func TestValidateWidgetAggregation_CampoVacioEnAgregacionNoCountFalla(t *testing.T) {
	for _, agg := range []models.AggregationType{models.AggSum, models.AggAvg, models.AggMin, models.AggMax} {
		w := models.Widget{Aggregation: agg, Field: ""}
		if err := validateWidgetAggregation(w); err == nil {
			t.Errorf("agregación %q sin campo debería fallar", agg)
		}
	}
}

func TestValidateWidgetAggregation_CountNoRequiereCampo(t *testing.T) {
	w := models.Widget{Aggregation: models.AggCount, Field: ""}
	if err := validateWidgetAggregation(w); err != nil {
		t.Errorf("count sin campo no debería fallar: %v", err)
	}
}

// AggDistinct existe en el modelo pero buildAggExpression nunca lo
// implementó — sin este chequeo, caía en "count" en silencio.
func TestValidateWidgetAggregation_AgregacionNoSoportadaFalla(t *testing.T) {
	w := models.Widget{Aggregation: models.AggDistinct, Field: "monto"}
	if err := validateWidgetAggregation(w); err == nil {
		t.Error("una agregación no implementada (distinct) debería rechazarse")
	}
}

func TestValidateWidgetAggregation_AgregacionesValidasConCampoPasan(t *testing.T) {
	for _, agg := range []models.AggregationType{models.AggCount, models.AggSum, models.AggAvg, models.AggMin, models.AggMax} {
		w := models.Widget{Aggregation: agg, Field: "monto"}
		if err := validateWidgetAggregation(w); err != nil {
			t.Errorf("agregación %q con campo no debería fallar: %v", agg, err)
		}
	}
}

package services

import (
	"fmt"
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// Un mcc que se ingirió como número nunca iguala al "7995" que manda el
// formulario: MongoDB no compara entre tipos BSON. Sin candidatos cruzados el
// filtro de una condición de velocidad descarta los cinco documentos y la
// regla no dispara nunca, sin un solo error en el log.
func TestConditionToMongoIgualdadCruzaTiposBSON(t *testing.T) {
	casos := []struct {
		nombre   string
		operador models.Operator
		valor    interface{}
		clave    string
		esperado []interface{}
	}{
		{"texto numerico con eq", models.OpEqual, "7995", "$in",
			[]interface{}{"7995", float64(7995)}},
		{"numero con eq", models.OpEqual, float64(7995), "$in",
			[]interface{}{float64(7995), "7995"}},
		{"texto numerico con neq", models.OpNotEqual, "7995", "$nin",
			[]interface{}{"7995", float64(7995)}},
		{"lista separada por comas con in", models.OpIn, "7995, 7994", "$in",
			[]interface{}{"7995", float64(7995), "7994", float64(7994)}},
		{"lista separada por comas con not_in", models.OpNotIn, "7995", "$nin",
			[]interface{}{"7995", float64(7995)}},
	}

	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			got := conditionToMongo(models.Condition{
				Field: "mcc", Operator: c.operador, Value: c.valor,
			})
			inner, ok := got["mcc"].(bson.M)
			if !ok {
				t.Fatalf("esperaba un operador sobre mcc, quedó %#v", got["mcc"])
			}
			cands, ok := inner[c.clave].([]interface{})
			if !ok {
				t.Fatalf("esperaba %s con lista de candidatos, quedó %#v", c.clave, inner)
			}
			if fmt.Sprint(cands) != fmt.Sprint(c.esperado) {
				t.Errorf("candidatos = %v, esperaba %v", cands, c.esperado)
			}
		})
	}
}

// Un identificador con ceros a la izquierda no es un número: convertirlo
// traería documentos que nadie pidió, que es el error opuesto y peor.
func TestConditionToMongoNoNumerizaTextoNoCanonico(t *testing.T) {
	for _, valor := range []string{"0012", "999000******4001", "", "1e5"} {
		got := conditionToMongo(models.Condition{
			Field: "tarjeta", Operator: models.OpEqual, Value: valor,
		})
		if got["tarjeta"] != interface{}(valor) {
			t.Errorf("con %q esperaba igualdad directa, quedó %#v", valor, got["tarjeta"])
		}
	}
}

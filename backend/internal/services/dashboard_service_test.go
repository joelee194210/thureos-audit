package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
)

// pipelineHasNumericTypeMatch reports whether the pipeline contains a
// {field: {$type: "number"}} match stage — the guard against $min/$max
// treating a stray string as the "max" over BSON type order.
func pipelineHasNumericTypeMatch(pipeline []bson.D, field string) bool {
	for _, stage := range pipeline {
		for _, elem := range stage {
			if elem.Key != "$match" {
				continue
			}
			match, ok := elem.Value.(bson.D)
			if !ok {
				continue
			}
			for _, m := range match {
				if m.Key != field {
					continue
				}
				cond, ok := m.Value.(bson.D)
				if !ok {
					continue
				}
				for _, c := range cond {
					if c.Key == "$type" && c.Value == "number" {
						return true
					}
				}
			}
		}
	}
	return false
}

// Antes de este cambio, $max sobre un campo mayormente numérico con un solo
// valor string colado devolvía ese string como "máximo" — BSON compara
// string > number, así que Mongo lo elegía sin quejarse. $min/$max necesitan
// filtrar los no numéricos antes de comparar; $sum/$avg ya lo hacen solos.
func TestBuildAggregationPipeline_MinMaxFiltraNoNumericos(t *testing.T) {
	for _, agg := range []models.AggregationType{models.AggMin, models.AggMax} {
		widget := models.Widget{Aggregation: agg, Field: "monto"}
		pipeline := buildAggregationPipeline(widget)
		if !pipelineHasNumericTypeMatch(pipeline, "monto") {
			t.Errorf("agregación %q debería filtrar 'monto' a solo valores numéricos antes de comparar", agg)
		}
	}
}

func TestBuildAggregationPipeline_SumAvgCountNoFiltranPorTipo(t *testing.T) {
	for _, agg := range []models.AggregationType{models.AggSum, models.AggAvg, models.AggCount} {
		widget := models.Widget{Aggregation: agg, Field: "monto"}
		pipeline := buildAggregationPipeline(widget)
		if pipelineHasNumericTypeMatch(pipeline, "monto") {
			t.Errorf("agregación %q no debería agregar el filtro de tipo numérico (ya ignora no numéricos por su cuenta)", agg)
		}
	}
}

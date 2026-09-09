package services

import (
	"time"

	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ChatAggregateSpec agrupa y agrega sin umbral — a diferencia de
// models.AggregateCondition (pensada para el motor de reglas, que corta
// por threshold), esta trae TODOS los grupos: el chatbot quiere el dato,
// no una decisión de match/no-match.
type ChatAggregateSpec struct {
	Field      string             `json:"field"`
	Function   models.AggFunction `json:"function"`
	GroupBy    string             `json:"groupBy,omitempty"`
	TimeField  string             `json:"timeField,omitempty"`
	TimeWindow string             `json:"timeWindow,omitempty"`
	Filter     []models.Condition `json:"filter,omitempty"`
}

// chatAggregateResultLimit topea cuántos grupos puede devolver una
// agregación del chatbot — evita que una pregunta sobre un campo de alta
// cardinalidad (ej. agrupar por ID de transacción) tire miles de grupos.
const chatAggregateResultLimit = 200

// buildDataAggregatePipeline arma el pipeline de agregación para el chat:
// mismos pasos que buildAggregatePipeline (ventana de tiempo, filtro
// previo, group) pero SIN el $match de threshold — el chatbot quiere
// todos los grupos, no solo los que superan un umbral.
func buildDataAggregatePipeline(spec ChatAggregateSpec) mongo.Pipeline {
	pipeline := mongo.Pipeline{}

	if spec.TimeField != "" && spec.TimeWindow != "" {
		dur := parseTimeWindow(spec.TimeWindow)
		if dur > 0 {
			// Fecha BSON, no string formateado — ver buildAggregatePipeline:
			// Mongo no compara entre tipos distintos y el filtro no matcheaba nada.
			cutoff := time.Now().Add(-dur)
			pipeline = append(pipeline, bson.D{
				{Key: "$match", Value: bson.D{
					{Key: spec.TimeField, Value: bson.D{
						{Key: "$gte", Value: cutoff},
					}},
				}},
			})
		}
	}

	if len(spec.Filter) > 0 {
		pipeline = append(pipeline, bson.D{
			{Key: "$match", Value: BuildMongoFilter(models.ConditionGroup{
				Logic: models.LogicAND, Conditions: spec.Filter,
			})},
		})
	}

	aggExpr := buildAggExpr(spec.Function, spec.Field)
	var groupID interface{}
	if spec.GroupBy != "" {
		groupID = "$" + spec.GroupBy
	}

	pipeline = append(pipeline, bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id", Value: groupID},
			{Key: "aggValue", Value: aggExpr},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		}},
	})

	pipeline = append(pipeline, bson.D{
		{Key: "$sort", Value: bson.D{{Key: "aggValue", Value: -1}}},
	})
	pipeline = append(pipeline, bson.D{
		{Key: "$limit", Value: chatAggregateResultLimit},
	})

	return pipeline
}

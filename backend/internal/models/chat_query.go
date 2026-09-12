package models

// ChatAggregateSpec agrupa y agrega sin umbral — a diferencia de
// AggregateCondition (pensada para el motor de reglas, que corta por
// threshold), esta trae TODOS los grupos: el chatbot quiere el dato, no
// una decisión de match/no-match.
//
// Vive en models y no en services porque ArtifactSource la persiste: un
// artefacto guarda la consulta que lo produjo, no sus números.
type ChatAggregateSpec struct {
	Field      string      `bson:"field" json:"field"`
	Function   AggFunction `bson:"function" json:"function"`
	GroupBy    string      `bson:"group_by,omitempty" json:"groupBy,omitempty"`
	TimeField  string      `bson:"time_field,omitempty" json:"timeField,omitempty"`
	TimeWindow string      `bson:"time_window,omitempty" json:"timeWindow,omitempty"`
	Filter     []Condition `bson:"filter,omitempty" json:"filter,omitempty"`
}

// ChatQueryInput es la forma exacta del "input" que el LLM manda al
// llamar a la herramienta query_monitor_data, y también lo que un
// ArtifactSource persiste.
type ChatQueryInput struct {
	// Monitor es el ALIAS del monitor a consultar, no su ObjectID.
	Monitor        string             `bson:"monitor" json:"monitor"`
	ConditionGroup *ConditionGroup    `bson:"condition_group,omitempty" json:"conditionGroup,omitempty"`
	Aggregate      *ChatAggregateSpec `bson:"aggregate,omitempty" json:"aggregate,omitempty"`
	Limit          int                `bson:"limit,omitempty" json:"limit,omitempty"`
}

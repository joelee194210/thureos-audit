package services

import (
	"fmt"
	"strings"

	"github.com/thureos/compliance/internal/models"
)

// TemplateField es un placeholder de campo dentro de una tipología. En la
// instanciación el usuario une cada placeholder a una columna real del
// esquema detectado del monitor.
type TemplateField string

const (
	FieldAmount      TemplateField = "$AMOUNT"
	FieldDate        TemplateField = "$DATE"
	FieldAccount     TemplateField = "$ACCOUNT"
	FieldCounterpart TemplateField = "$COUNTERPART"
	FieldCountry     TemplateField = "$COUNTRY"
	FieldMCC         TemplateField = "$MCC"
)

// TemplateParam es un umbral ajustable de una tipología.
type TemplateParam struct {
	Key         string  `json:"key"`
	Label       string  `json:"label"`
	Description string  `json:"description"`
	Default     float64 `json:"default"`
	Min         float64 `json:"min"`
}

// RuleTemplate es una tipología AML pre-armada: metadatos para la galería
// y un constructor que produce una models.Rule a partir del mapeo de
// campos y los parámetros elegidos. La regla devuelta no trae ID,
// MonitorID, Name ni CreatedBy — eso los fija el caller.
type RuleTemplate struct {
	ID                string          `json:"id"`
	Name              string          `json:"name"`
	Description       string          `json:"description"`
	SuggestedSeverity models.Severity `json:"suggestedSeverity"`
	RequiredFields    []TemplateField `json:"requiredFields"`
	Params            []TemplateParam `json:"params"`
	build             func(fields map[TemplateField]string, params map[string]float64) (models.Rule, error)
}

// BindTemplate instancia una tipología: valida el mapeo de campos,
// resuelve los parámetros (con defaults) y devuelve la regla lista para
// persistir. name y monitorID los aporta el caller.
func BindTemplate(t RuleTemplate, fieldMap map[string]string, params map[string]float64) (models.Rule, error) {
	fields, err := resolveFields(fieldMap, t.RequiredFields)
	if err != nil {
		return models.Rule{}, err
	}
	rule, err := t.build(fields, params)
	if err != nil {
		return models.Rule{}, fmt.Errorf("construyendo tipología %s: %w", t.ID, err)
	}
	rule.Severity = t.SuggestedSeverity
	rule.Description = t.Description
	rule.Actions = []models.ActionType{models.ActionRedFlag}
	return rule, nil
}

// resolveFields exige que cada campo requerido esté mapeado a una columna
// no vacía. Rechaza con la lista completa de faltantes, no con la primera.
func resolveFields(fieldMap map[string]string, required []TemplateField) (map[TemplateField]string, error) {
	var missing []string
	resolved := make(map[TemplateField]string, len(required))
	for _, f := range required {
		col := strings.TrimSpace(fieldMap[string(f)])
		if col == "" {
			missing = append(missing, string(f))
			continue
		}
		resolved[f] = col
	}
	if len(missing) > 0 {
		return nil, fmt.Errorf("faltan mapeos de campo: %s", strings.Join(missing, ", "))
	}
	return resolved, nil
}

// paramOr devuelve el valor elegido por el usuario o el default de la
// tipología. Un valor <= min se descarta (umbral sin sentido, p.ej. 0).
func paramOr(params map[string]float64, p TemplateParam) float64 {
	if v, ok := params[p.Key]; ok && v >= p.Min {
		return v
	}
	return p.Default
}

func param(key, label, description string, def, min float64) TemplateParam {
	return TemplateParam{Key: key, Label: label, Description: description, Default: def, Min: min}
}

func simpleCondition(field string, op models.Operator, value interface{}) models.Condition {
	return models.Condition{Field: field, Operator: op, Value: value}
}

func aggCondition(field, groupBy, timeField string, fn models.AggFunction, window string, op models.Operator, threshold float64) models.AggregateCondition {
	return models.AggregateCondition{
		Field:      string(field),
		Function:   fn,
		GroupBy:    string(groupBy),
		TimeField:  string(timeField),
		TimeWindow: window,
		Operator:   op,
		Threshold:  threshold,
	}
}

// Listas por defecto de alto riesgo. Son punto de partida del catálogo,
// editables por el cliente desde la regla instanciada; los umbrales
// numéricos sí son parámetros del template.
var (
	defaultHighRiskCountries = []string{"AF", "IR", "KP", "SY", "MM", "PA", "CY", "HT"}
	defaultHighRiskMCCs      = []string{"4826", "6051", "6012", "7995", "7800", "5993", "9399"}
)

var pMaxTx24h = param("maxTx", "Máximo de transacciones", "Más transacciones que esta cantidad en 24h por cuenta dispara la alerta", 10, 1)
var pMaxTx7d = param("maxTx", "Máximo de transacciones", "Más transacciones que esta cantidad en 7 días por cuenta dispara la alerta", 30, 1)
var pAmountThreshold = param("amount", "Umbral de monto", "Monto a partir del cual se dispara la alerta", 10000, 0.01)
var pSumThreshold = param("sum", "Umbral de suma acumulada", "Suma acumulada a partir de la cual se dispara la alerta", 50000, 0.01)
var pBandFloor = param("floor", "Piso de la franja", "Monto mínimo de la franja de estructuración", 1000, 0)
var pBandCeil = param("ceil", "Techo de la franja", "Monto máximo de la franja (típicamente el umbral de reporte)", 10000, 0.01)

// RuleTemplates es el catálogo v1. Cada template es puramente fila o
// puramente agregado: el engine evalúa ambos artefactos por separado y
// cada uno dispara su propia red flag, así que una tipología no mezcla.
var RuleTemplates = []RuleTemplate{
	{
		ID:                "velocity_24h",
		Name:              "Velocidad de transacciones (24h)",
		Description:       "Una cuenta concentra más transacciones que el umbral en las últimas 24 horas. Patrón típico de cuenta mula o fraccionamiento.",
		SuggestedSeverity: models.SeverityMedium,
		RequiredFields:    []TemplateField{FieldAccount},
		Params:            []TemplateParam{pMaxTx24h},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				AggregateConditions: []models.AggregateCondition{
					aggCondition(f[FieldAccount], f[FieldAccount], f[FieldDate], models.AggFuncCount, "24h", models.OpGreaterThan, paramOr(p, pMaxTx24h)),
				},
			}, nil
		},
	},
	{
		ID:                "accumulated_sum_7d",
		Name:              "Suma acumulada por cuenta (7d)",
		Description:       "La suma de transacciones de una cuenta supera el umbral en los últimos 7 días. Detecta movimiento de volumen anómalo.",
		SuggestedSeverity: models.SeverityHigh,
		RequiredFields:    []TemplateField{FieldAmount, FieldAccount, FieldDate},
		Params:            []TemplateParam{pSumThreshold},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				AggregateConditions: []models.AggregateCondition{
					aggCondition(f[FieldAmount], f[FieldAccount], f[FieldDate], models.AggFuncSum, "7d", models.OpGreaterThan, paramOr(p, pSumThreshold)),
				},
			}, nil
		},
	},
	{
		ID:                "structuring_band",
		Name:              "Franja de estructuración",
		Description:       "Transacciones cuyo monto cae en la franja típica de fraccionamiento (justo debajo del umbral de reporte). Versión por fila; la correlación por cuenta llega con agregados filtrados.",
		SuggestedSeverity: models.SeverityHigh,
		RequiredFields:    []TemplateField{FieldAmount},
		Params:            []TemplateParam{pBandFloor, pBandCeil},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				ConditionGroup: models.ConditionGroup{
					Logic: models.LogicAND,
					Conditions: []models.Condition{
						simpleCondition(f[FieldAmount], models.OpGreaterEqual, paramOr(p, pBandFloor)),
						simpleCondition(f[FieldAmount], models.OpLessThan, paramOr(p, pBandCeil)),
					},
				},
			}, nil
		},
	},
	{
		ID:                "high_amount",
		Name:              "Transacción de alto monto",
		Description:       "Cualquier transacción individual que supera el umbral configurado.",
		SuggestedSeverity: models.SeverityMedium,
		RequiredFields:    []TemplateField{FieldAmount},
		Params:            []TemplateParam{pAmountThreshold},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				ConditionGroup: models.ConditionGroup{
					Logic: models.LogicAND,
					Conditions: []models.Condition{
						simpleCondition(f[FieldAmount], models.OpGreaterThan, paramOr(p, pAmountThreshold)),
					},
				},
			}, nil
		},
	},
	{
		ID:                "avg_anomaly_30d",
		Name:              "Ticket promedio anómalo (30d)",
		Description:       "El ticket promedio de una cuenta supera el umbral en los últimos 30 días: cambio de patrón de gasto.",
		SuggestedSeverity: models.SeverityMedium,
		RequiredFields:    []TemplateField{FieldAmount, FieldAccount, FieldDate},
		Params:            []TemplateParam{pAmountThreshold},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				AggregateConditions: []models.AggregateCondition{
					aggCondition(f[FieldAmount], f[FieldAccount], f[FieldDate], models.AggFuncAvg, "30d", models.OpGreaterThan, paramOr(p, pAmountThreshold)),
				},
			}, nil
		},
	},
	{
		ID:                "max_spike_30d",
		Name:              "Pico de máximo (30d)",
		Description:       "La mayor transacción de una cuenta en 30 días supera el umbral: captura picos que el promedio diluye.",
		SuggestedSeverity: models.SeverityMedium,
		RequiredFields:    []TemplateField{FieldAmount, FieldAccount, FieldDate},
		Params:            []TemplateParam{pAmountThreshold},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				AggregateConditions: []models.AggregateCondition{
					aggCondition(f[FieldAmount], f[FieldAccount], f[FieldDate], models.AggFuncMax, "30d", models.OpGreaterThan, paramOr(p, pAmountThreshold)),
				},
			}, nil
		},
	},
	{
		ID:                "high_risk_country",
		Name:              "País de alto riesgo",
		Description:       "Transacciones originadas o destinadas a países de la lista por defecto (FATF/alto riesgo). La lista queda editable en la regla creada.",
		SuggestedSeverity: models.SeverityHigh,
		RequiredFields:    []TemplateField{FieldCountry},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				ConditionGroup: models.ConditionGroup{
					Logic: models.LogicAND,
					Conditions: []models.Condition{
						simpleCondition(f[FieldCountry], models.OpIn, defaultHighRiskCountries),
					},
				},
			}, nil
		},
	},
	{
		ID:                "high_risk_mcc",
		Name:              "MCC de alto riesgo",
		Description:       "Transacciones en rubros de alto riesgo (transferencias de dinero, quasi-cash, juego). Lista editable en la regla creada.",
		SuggestedSeverity: models.SeverityHigh,
		RequiredFields:    []TemplateField{FieldMCC},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				ConditionGroup: models.ConditionGroup{
					Logic: models.LogicAND,
					Conditions: []models.Condition{
						simpleCondition(f[FieldMCC], models.OpIn, defaultHighRiskMCCs),
					},
				},
			}, nil
		},
	},
	{
		ID:                "counterpart_accumulation_30d",
		Name:              "Concentración en contraparte (30d)",
		Description:       "Una misma contraparte concentra suma de transacciones por encima del umbral en 30 días. Detecta pagos fragmentados a un beneficiario.",
		SuggestedSeverity: models.SeverityHigh,
		RequiredFields:    []TemplateField{FieldAmount, FieldCounterpart, FieldDate},
		Params:            []TemplateParam{pSumThreshold},
		build: func(f map[TemplateField]string, p map[string]float64) (models.Rule, error) {
			return models.Rule{
				AggregateConditions: []models.AggregateCondition{
					aggCondition(f[FieldAmount], f[FieldCounterpart], f[FieldDate], models.AggFuncSum, "30d", models.OpGreaterThan, paramOr(p, pSumThreshold)),
				},
			}, nil
		},
	},
}

// FindRuleTemplate devuelve la tipología por ID.
func FindRuleTemplate(id string) (RuleTemplate, bool) {
	for _, t := range RuleTemplates {
		if t.ID == id {
			return t, true
		}
	}
	return RuleTemplate{}, false
}

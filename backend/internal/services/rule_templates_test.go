package services

import (
	"strings"
	"testing"

	"github.com/thureos/compliance/internal/models"
)

// El catálogo completo tiene que instanciarse sin error: cada tipología
// construye una regla válida cuando todos sus campos están mapeados.
func TestCatalogo_TodasLasTipologiasConstruyen(t *testing.T) {
	ids := map[string]bool{}
	for _, tpl := range RuleTemplates {
		if ids[tpl.ID] {
			t.Errorf("ID duplicado en catálogo: %s", tpl.ID)
		}
		ids[tpl.ID] = true

		fieldMap := map[string]string{}
		for _, f := range tpl.RequiredFields {
			fieldMap[string(f)] = "columna_" + strings.TrimPrefix(string(f), "$")
		}
		rule, err := BindTemplate(tpl, fieldMap, nil)
		if err != nil {
			t.Errorf("%s: BindTemplate inesperadamente falló: %v", tpl.ID, err)
			continue
		}
		if rule.Severity != tpl.SuggestedSeverity {
			t.Errorf("%s: severity = %v, want %v", tpl.ID, rule.Severity, tpl.SuggestedSeverity)
		}
		if len(rule.Actions) != 1 || rule.Actions[0] != models.ActionRedFlag {
			t.Errorf("%s: actions = %v, want [red_flag]", tpl.ID, rule.Actions)
		}
		if rule.Description == "" {
			t.Errorf("%s: description vacía", tpl.ID)
		}
		// cada template es puro-fila o puro-agregado: el engine evalúa
		// ambos por separado y dispararía dos red flags desincronizadas
		hasConds := len(rule.ConditionGroup.Conditions) > 0
		hasAggs := len(rule.AggregateConditions) > 0
		if hasConds == hasAggs {
			t.Errorf("%s: debe ser puro-fila o puro-agregado (conds=%v, aggs=%v)", tpl.ID, hasConds, hasAggs)
		}
	}
}

func TestBindTemplate_VelocidadConDefaults(t *testing.T) {
	tpl, ok := FindRuleTemplate("velocity_24h")
	if !ok {
		t.Fatal("template velocity_24h no existe en el catálogo")
	}
	rule, err := BindTemplate(tpl, map[string]string{"$ACCOUNT": "cuenta_origen", "$DATE": "fecha"}, nil)
	if err != nil {
		t.Fatalf("BindTemplate: %v", err)
	}
	if len(rule.AggregateConditions) != 1 {
		t.Fatalf("agregados = %d, want 1", len(rule.AggregateConditions))
	}
	agg := rule.AggregateConditions[0]
	if agg.Function != models.AggFuncCount {
		t.Errorf("function = %v, want count", agg.Function)
	}
	if agg.Field != "cuenta_origen" || agg.GroupBy != "cuenta_origen" {
		t.Errorf("field/groupBy = %s/%s, want cuenta_origen/cuenta_origen", agg.Field, agg.GroupBy)
	}
	if agg.TimeWindow != "24h" {
		t.Errorf("window = %v, want 24h", agg.TimeWindow)
	}
	if agg.Threshold != 10 {
		t.Errorf("threshold = %v, want default 10", agg.Threshold)
	}
}

func TestBindTemplate_ParametrosSobrescribenDefaults(t *testing.T) {
	tpl, _ := FindRuleTemplate("accumulated_sum_7d")
	rule, err := BindTemplate(tpl,
		map[string]string{"$AMOUNT": "monto", "$ACCOUNT": "cliente", "$DATE": "fecha_op"},
		map[string]float64{"sum": 250000})
	if err != nil {
		t.Fatalf("BindTemplate: %v", err)
	}
	if got := rule.AggregateConditions[0].Threshold; got != 250000 {
		t.Errorf("threshold = %v, want 250000", got)
	}
	if rule.AggregateConditions[0].TimeField != "fecha_op" {
		t.Errorf("timeField = %v, want fecha_op", rule.AggregateConditions[0].TimeField)
	}
}

// Un parámetro por debajo del mínimo se descarta y cae al default:
// protege contra "0 transacciones dispara todo".
func TestBindTemplate_ParametroBajoMinimoCaeADefault(t *testing.T) {
	tpl, _ := FindRuleTemplate("velocity_24h")
	rule, err := BindTemplate(tpl,
		map[string]string{"$ACCOUNT": "cta", "$DATE": "fecha"},
		map[string]float64{"maxTx": 0})
	if err != nil {
		t.Fatalf("BindTemplate: %v", err)
	}
	if got := rule.AggregateConditions[0].Threshold; got != 10 {
		t.Errorf("threshold = %v, want default 10 (0 está bajo min)", got)
	}
}

// El error de faltantes lista TODOS los placeholders sin mapear (no solo
// el primero) para que el usuario corrija de una sola vez.
func TestBindTemplate_FaltanMapeosListaCompleta(t *testing.T) {
	tpl, _ := FindRuleTemplate("accumulated_sum_7d")
	_, err := BindTemplate(tpl, map[string]string{"$AMOUNT": "monto"}, nil)
	if err == nil {
		t.Fatal("esperaba error por mapeos faltantes")
	}
	for _, f := range []string{"$ACCOUNT", "$DATE"} {
		if !strings.Contains(err.Error(), f) {
			t.Errorf("error %q no menciona %s", err.Error(), f)
		}
	}
}

func TestBindTemplate_ColumnaVaciaRechazada(t *testing.T) {
	tpl, _ := FindRuleTemplate("high_amount")
	_, err := BindTemplate(tpl, map[string]string{"$AMOUNT": "   "}, nil)
	if err == nil {
		t.Fatal("esperaba error por columna vacía/whitespace")
	}
}

func TestBindTemplate_CondicionFilaConUmbralElegido(t *testing.T) {
	tpl, _ := FindRuleTemplate("high_amount")
	rule, err := BindTemplate(tpl, map[string]string{"$AMOUNT": "importe"}, map[string]float64{"amount": 500})
	if err != nil {
		t.Fatalf("BindTemplate: %v", err)
	}
	if len(rule.AggregateConditions) != 0 {
		t.Errorf("agregados = %d, want 0 (template puro-fila)", len(rule.AggregateConditions))
	}
	conds := rule.ConditionGroup.Conditions
	if len(conds) != 1 || conds[0].Operator != models.OpGreaterThan || conds[0].Value != 500.0 {
		t.Errorf("condiciones = %+v, want [importe gt 500]", conds)
	}
}

func TestBindTemplate_ListasPorDefectoEnCondicionIn(t *testing.T) {
	tpl, _ := FindRuleTemplate("high_risk_country")
	rule, err := BindTemplate(tpl, map[string]string{"$COUNTRY": "pais_residencia"}, nil)
	if err != nil {
		t.Fatalf("BindTemplate: %v", err)
	}
	conds := rule.ConditionGroup.Conditions
	if len(conds) != 1 || conds[0].Operator != models.OpIn {
		t.Fatalf("condiciones = %+v, want [pais gt/e ...]", conds)
	}
	list, ok := conds[0].Value.([]string)
	if !ok || len(list) == 0 {
		t.Errorf("value de 'in' = %#v, want lista de países", conds[0].Value)
	}
}

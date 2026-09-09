package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// aggFuncionesValidas son las que buildAggExpr sabe traducir. Cualquier otra
// cae a su `default: $sum`, así que el umbral termina comparándose contra
// una cantidad distinta de la que el usuario pidió, sin ningún error.
var aggFuncionesValidas = map[models.AggFunction]bool{
	models.AggFuncSum:   true,
	models.AggFuncCount: true,
	models.AggFuncAvg:   true,
	models.AggFuncMin:   true,
	models.AggFuncMax:   true,
}

// ValidateAggregateCondition rechaza al guardar las condiciones agregadas
// que no podrían disparar nunca. Es deliberadamente el mismo criterio que
// discardReason aplica a las sugerencias de IA: lo que la IA no puede
// sugerir tampoco se puede escribir a mano desde el editor.
//
// El chequeo de tipo sobre TimeField es el que más importa: la ventana se
// arma con un $match contra un time.Time (rule_engine.go, buildAggregatePipeline)
// y MongoDB no compara entre tipos BSON, así que una ventana sobre un campo
// numérico descarta todos los documentos en silencio.
func ValidateAggregateCondition(cond models.AggregateCondition, schema []models.SchemaField) error {
	byName := make(map[string]models.SchemaField, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}

	if !aggFuncionesValidas[cond.Function] {
		return fmt.Errorf("función de agregación inválida: %q (válidas: sum, count, avg, min, max)", cond.Function)
	}

	// Para count el motor ignora Field (buildAggExpr emite $sum:1).
	if cond.Function != models.AggFuncCount && cond.Field == "" {
		return fmt.Errorf("el agregado no dice qué campo agregar")
	}
	if cond.Field != "" {
		if _, ok := byName[cond.Field]; !ok {
			return fmt.Errorf("el campo a agregar %q no existe en el esquema del monitor", cond.Field)
		}
	}

	// Un groupBy vacío es un agregado global: el motor agrupa con _id:null.
	if cond.GroupBy != "" {
		if _, ok := byName[cond.GroupBy]; !ok {
			return fmt.Errorf("el campo de agrupación %q no existe en el esquema del monitor", cond.GroupBy)
		}
	}

	// Media ventana es intención a medio expresar: sin el par completo el
	// motor ignora la ventana y "5 en 60s" se vuelve "5 alguna vez".
	if (cond.TimeField == "") != (cond.TimeWindow == "") {
		return fmt.Errorf("la ventana de tiempo está incompleta: hacen falta el campo de fecha y la duración")
	}
	if cond.TimeField != "" {
		f, ok := byName[cond.TimeField]
		if !ok {
			return fmt.Errorf("el campo de tiempo %q no existe en el esquema del monitor", cond.TimeField)
		}
		if f.Type != models.FieldDate {
			return fmt.Errorf("el campo de tiempo %q es de tipo %s: se necesita un campo date", cond.TimeField, f.Type)
		}
		// Mismo chequeo de unidad que discardReason aplica a las sugerencias de
		// IA (validAITimeWindow, ai_rules_service.go): "m" (meses) no está en el
		// vocabulario enseñado y se trata como ambiguo en vez de arriesgar que
		// signifique otra cosa para quien la escribió a mano.
		if !validAITimeWindow.MatchString(cond.TimeWindow) {
			return fmt.Errorf("la ventana %q no usa una unidad válida (s, min, h, d)", cond.TimeWindow)
		}
		// "0s"/"0d" pasan esa validación de formato pero parsean a cero:
		// buildAggregatePipeline se salta el $match de ventana entero y "5 en
		// 0s" se vuelve silenciosamente "5 alguna vez".
		if parseTimeWindow(cond.TimeWindow) <= 0 {
			return fmt.Errorf("ventana de tiempo inválida: %q (formatos válidos: 35s, 5min, 24h, 7d)", cond.TimeWindow)
		}
	}

	for _, f := range cond.Filter {
		if _, ok := byName[f.Field]; !ok {
			return fmt.Errorf("el filtro del agregado referencia un campo inexistente: %q", f.Field)
		}
	}

	return nil
}

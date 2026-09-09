package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// ValidateVelocityCondition rechaza al guardar las condiciones que no
// podrían disparar nunca. El precedente que justifica esta severidad: la
// ventana temporal rota vivió meses en producción porque fallaba en silencio
// al evaluar en vez de rechazarse al configurarse.
func ValidateVelocityCondition(cond models.VelocityCondition, schema []models.SchemaField) error {
	byName := make(map[string]models.SchemaField, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}

	if cond.TimeField == "" {
		return fmt.Errorf("la condición de velocidad necesita un campo de tiempo")
	}
	timeField, ok := byName[cond.TimeField]
	if !ok {
		return fmt.Errorf("el campo de tiempo %q no existe en el esquema del monitor", cond.TimeField)
	}
	if timeField.Type != models.FieldDate {
		return fmt.Errorf("el campo de tiempo %q es de tipo %s: se necesita un campo date", cond.TimeField, timeField.Type)
	}
	if parseTimeWindow(cond.MaxGap) <= 0 {
		return fmt.Errorf("gap máximo inválido: %q (formatos válidos: 35s, 5min, 24h, 7d)", cond.MaxGap)
	}
	if cond.GroupBy != "" {
		if _, ok := byName[cond.GroupBy]; !ok {
			return fmt.Errorf("el campo de agrupación %q no existe en el esquema del monitor", cond.GroupBy)
		}
	}
	if cond.MinEvents < 2 {
		return fmt.Errorf("el mínimo de eventos debe ser al menos 2: hace falta un par para medir un gap")
	}
	// El pipeline mide gaps entre eventos consecutivos, o sea pares. Exigir
	// rachas más largas requeriría contar gaps consecutivos por partición,
	// que todavía no está implementado — se rechaza en vez de aceptarlo y
	// comportarse como si fuera 2.
	if cond.MinEvents > 2 {
		return fmt.Errorf("por ahora solo se soporta minEvents = 2 (un par de transacciones consecutivas); rachas más largas todavía no están implementadas")
	}
	for _, f := range cond.Filter {
		if _, ok := byName[f.Field]; !ok {
			return fmt.Errorf("el filtro referencia un campo inexistente: %q", f.Field)
		}
	}
	return nil
}

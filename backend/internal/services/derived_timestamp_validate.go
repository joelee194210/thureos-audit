package services

import (
	"fmt"

	"github.com/thureos/compliance/internal/models"
)

// ValidateDerivedTimestamp rechaza configuraciones que no podrían funcionar,
// en el momento de guardarlas. Una configuración inválida que se acepta y
// falla en silencio al ingerir es exactamente el modo de falla que dejó la
// ventana temporal rota en producción sin que nadie se enterara.
func ValidateDerivedTimestamp(cfg models.DerivedTimestampConfig, schema []models.SchemaField) error {
	names := make(map[string]bool, len(schema))
	for _, f := range schema {
		names[f.Name] = true
	}

	if !names[cfg.DateField] {
		return fmt.Errorf("el campo de fecha %q no existe en el esquema del monitor", cfg.DateField)
	}
	if !names[cfg.TimeField] {
		return fmt.Errorf("el campo de hora %q no existe en el esquema del monitor", cfg.TimeField)
	}
	if _, ok := timestampLayouts[cfg.DateFormat]; !ok {
		return fmt.Errorf("formato de fecha no soportado: %q", cfg.DateFormat)
	}
	if _, ok := timestampLayouts[cfg.TimeFormat]; !ok {
		return fmt.Errorf("formato de hora no soportado: %q", cfg.TimeFormat)
	}
	if cfg.TargetName == "" {
		return fmt.Errorf("el nombre del campo derivado no puede estar vacío")
	}
	if names[cfg.TargetName] {
		return fmt.Errorf("el nombre %q ya es una columna del monitor", cfg.TargetName)
	}
	return nil
}

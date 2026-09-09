package services

import "github.com/thureos/compliance/internal/models"

// SchemaAfterApproval devuelve el schema que hay que persistir después de
// aprobar una carga rechazada por estructura, y si hubo conflicto de nombres.
//
// Aprobar resetea el schema del monitor al del archivo aprobado, así que sin
// esto el campo derivado desaparecía del schema mientras su configuración
// seguía viva: las reglas que lo usaban dejaban de validar y el campo se caía
// de los selectores, aunque los documentos ingeridos sí lo llevaban.
//
// El conflicto (el archivo aprobado trae de verdad una columna con el nombre
// del campo derivado) se reporta en vez de resolverse acá: pisar una columna
// real del usuario con un campo calculado sería peor que cualquier error.
func SchemaAfterApproval(detected []models.SchemaField, cfg *models.DerivedTimestampConfig) ([]models.SchemaField, bool) {
	if cfg == nil || cfg.TargetName == "" {
		return detected, false
	}

	for _, f := range detected {
		if f.Name == cfg.TargetName {
			return detected, true
		}
	}

	return append(detected, models.SchemaField{
		Name:     cfg.TargetName,
		Type:     models.FieldDate,
		Required: false,
	}), false
}

package services

import (
	"fmt"
	"strings"

	"github.com/thureos/compliance/internal/models"
)

// ValidateDerivedTimestamp rechaza configuraciones que no podrían funcionar,
// en el momento de guardarlas. Una configuración inválida que se acepta y
// falla en silencio al ingerir es exactamente el modo de falla que dejó la
// ventana temporal rota en producción sin que nadie se enterara.
func ValidateDerivedTimestamp(cfg models.DerivedTimestampConfig, schema []models.SchemaField) error {
	byName := make(map[string]models.SchemaField, len(schema))
	for _, f := range schema {
		byName[f.Name] = f
	}

	dateField, ok := byName[cfg.DateField]
	if !ok {
		return fmt.Errorf("el campo de fecha %q no existe en el esquema del monitor", cfg.DateField)
	}
	timeField, ok := byName[cfg.TimeField]
	if !ok {
		return fmt.Errorf("el campo de hora %q no existe en el esquema del monitor", cfg.TimeField)
	}
	if err := validateDerivedSourceField(dateField, "fecha"); err != nil {
		return err
	}
	if err := validateDerivedSourceField(timeField, "hora"); err != nil {
		return err
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
	if err := validateDerivedTargetName(cfg.TargetName); err != nil {
		return err
	}
	if _, ok := byName[cfg.TargetName]; ok {
		return fmt.Errorf("el nombre %q ya es una columna del monitor", cfg.TargetName)
	}
	return nil
}

// validateDerivedTargetName rechaza los nombres que romperían el documento
// en vez de agregarle un campo. El schema del monitor no contiene los campos
// internos (_id, _ingested_at), así que sin esta comprobación pasaban la
// validación de colisión y después pisaban el documento: "_ingested_at" es
// el campo del que dependen el scheduler y toda la evaluación date-scoped
// —romperlo rompe TODAS las reglas del monitor, no solo la nueva— y "_id" es
// inmutable, así que el backfill y la ingesta fallan. Un "." crea una ruta
// anidada y un "$" rompe la referencia "$nombre" que arma el pipeline.
func validateDerivedTargetName(name string) error {
	if strings.HasPrefix(name, "_") {
		return fmt.Errorf(
			"el nombre %q no puede empezar con guion bajo: esos nombres están reservados para campos internos del documento (_id, _ingested_at)",
			name,
		)
	}
	if strings.ContainsAny(name, ".$") {
		return fmt.Errorf("el nombre %q no puede contener '.' ni '$'", name)
	}
	return nil
}

// validateDerivedSourceField exige que el campo fuente sea legible por
// padNumericField: un número o un string numérico, sin decimales implícitos.
// Un campo tipado date llega a padNumericField como time.Time (parseValue ya
// lo convirtió) y cae en el default: no se construye ningún timestamp. Uno
// con decimales implícitos convierte 20260907 en 202609.07, que tampoco
// parsea. En ambos casos la configuración se aceptaba, el campo aparecía en
// el schema, una regla de velocidad validaba contra él, y ningún documento
// lo llevaba nunca.
func validateDerivedSourceField(f models.SchemaField, rol string) error {
	if f.Type != models.FieldNumber && f.Type != models.FieldString {
		return fmt.Errorf(
			"el campo de %s %q es de tipo %s: se necesita un campo numérico o de texto (un campo date ya viene convertido y nunca produciría el timestamp)",
			rol, f.Name, f.Type,
		)
	}
	if f.ImpliedDecimals > 0 {
		return fmt.Errorf(
			"el campo de %s %q tiene %d decimales implícitos: el valor se dividiría por una potencia de 10 y nunca produciría el timestamp",
			rol, f.Name, f.ImpliedDecimals,
		)
	}
	return nil
}

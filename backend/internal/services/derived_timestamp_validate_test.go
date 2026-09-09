package services

import (
	"testing"

	"github.com/thureos/compliance/internal/models"
)

func schemaConFechaYHora() []models.SchemaField {
	return []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "horapoliza", Type: models.FieldNumber},
		{Name: "importe", Type: models.FieldNumber},
	}
}

func TestValidateDerivedTimestamp_ConfigValida(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err != nil {
		t.Errorf("esperaba configuración válida, got %v", err)
	}
}

func TestValidateDerivedTimestamp_CampoInexistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "no_existe", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un campo fuera del esquema debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_FormatoNoSoportado(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "DD/MM/YYYY",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un formato fuera de los soportados debería rechazarse")
	}
}

// El campo derivado no puede pisar una columna real del archivo.
func TestValidateDerivedTimestamp_TargetColisionaConCampoExistente(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "importe",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName que pisa un campo existente debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_TargetVacio(t *testing.T) {
	cfg := models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "",
	}
	if err := ValidateDerivedTimestamp(cfg, schemaConFechaYHora()); err == nil {
		t.Error("un TargetName vacío debería rechazarse")
	}
}

func cfgConTarget(target string) models.DerivedTimestampConfig {
	return models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: target,
	}
}

// I2: el conjunto de nombres se armaba solo con monitor.Schema, que nunca
// contiene los campos internos del documento. "_ingested_at" pisaría el
// campo del que dependen la evaluación date-scoped y el scheduler — rompe
// TODAS las reglas del monitor, no solo la nueva. "_id" es inmutable: el
// backfill y la ingesta fallarían. Un "." o un "$" crean una ruta anidada y
// una referencia "$nombre" rota en el pipeline.
func TestValidateDerivedTimestamp_TargetNameReservadoOInvalido(t *testing.T) {
	casos := []struct {
		nombre string
		porque string
	}{
		{"_ingested_at", "pisa el campo del que dependen el scheduler y el camino date-scoped"},
		{"_id", "es inmutable: rompe el backfill y la ingesta"},
		{"_interno", "los nombres con guion bajo inicial quedan reservados para campos internos"},
		{"fecha.hora", "un punto crea una ruta anidada en vez de un campo"},
		{"fecha$hora", "un $ rompe la referencia \"$nombre\" del pipeline"},
		{"$timestamp", "un $ rompe la referencia \"$nombre\" del pipeline"},
	}
	for _, caso := range casos {
		t.Run(caso.nombre, func(t *testing.T) {
			if err := ValidateDerivedTimestamp(cfgConTarget(caso.nombre), schemaConFechaYHora()); err == nil {
				t.Errorf("%q debería rechazarse: %s", caso.nombre, caso.porque)
			}
		})
	}
}

// I3: los campos fuente se leen con padNumericField, que solo entiende
// números y strings numéricos. Un campo tipado date lo hace devolver false
// (parseValue ya lo convirtió en time.Time) y un campo con decimales
// implícitos convierte 20260907 en 202609.07: la configuración se aceptaba,
// el campo aparecía en el schema, y ningún documento lo llevaba nunca.
func TestValidateDerivedTimestamp_CampoFuenteTipoDate(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldDate, DateFormat: "YYYYMMDD"},
		{Name: "horapoliza", Type: models.FieldNumber},
	}
	if err := ValidateDerivedTimestamp(cfgConTarget("timestamp"), schema); err == nil {
		t.Error("un campo fuente tipado date nunca produce un timestamp: debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_CampoFuenteBooleano(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldNumber},
		{Name: "horapoliza", Type: models.FieldBoolean},
	}
	if err := ValidateDerivedTimestamp(cfgConTarget("timestamp"), schema); err == nil {
		t.Error("un campo fuente booleano nunca produce un timestamp: debería rechazarse")
	}
}

func TestValidateDerivedTimestamp_CampoFuenteConDecimalesImplicitos(t *testing.T) {
	for _, campo := range []string{"fechapoliza", "horapoliza"} {
		t.Run(campo, func(t *testing.T) {
			schema := []models.SchemaField{
				{Name: "fechapoliza", Type: models.FieldNumber},
				{Name: "horapoliza", Type: models.FieldNumber},
			}
			for i := range schema {
				if schema[i].Name == campo {
					schema[i].ImpliedDecimals = 2
				}
			}
			if err := ValidateDerivedTimestamp(cfgConTarget("timestamp"), schema); err == nil {
				t.Errorf("%q con decimales implícitos nunca produce un timestamp: debería rechazarse", campo)
			}
		})
	}
}

// Un campo fuente de tipo string sí sirve: padNumericField acepta strings
// numéricos (es el caso de una columna con ceros a la izquierda).
func TestValidateDerivedTimestamp_CampoFuenteStringEsValido(t *testing.T) {
	schema := []models.SchemaField{
		{Name: "fechapoliza", Type: models.FieldString},
		{Name: "horapoliza", Type: models.FieldString},
	}
	if err := ValidateDerivedTimestamp(cfgConTarget("timestamp"), schema); err != nil {
		t.Errorf("un campo fuente string debería aceptarse, got %v", err)
	}
}

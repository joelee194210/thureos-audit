package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/models"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
)

func TestDetectSchemaFromJSON_DetectsDatesMixedWithStringsAndNumbers(t *testing.T) {
	records := []map[string]interface{}{
		{
			"amount":       1200.50,
			"createdAt":    "2024-01-15",
			"updatedAt":    "2024-01-15T10:30:00Z",
			"description":  "wire transfer",
			"isReconciled": true,
		},
	}

	schema := detectSchemaFromJSON(records)

	types := make(map[string]models.FieldType, len(schema))
	for _, f := range schema {
		types[f.Name] = f.Type
	}

	cases := map[string]models.FieldType{
		"amount":       models.FieldNumber,
		"createdAt":    models.FieldDate,
		"updatedAt":    models.FieldDate,
		"description":  models.FieldString,
		"isReconciled": models.FieldBoolean,
	}
	for field, want := range cases {
		got, ok := types[field]
		if !ok {
			t.Fatalf("field %q missing from detected schema: %+v", field, schema)
		}
		if got != want {
			t.Errorf("field %q: got type %q, want %q", field, got, want)
		}
	}
	if len(schema) != len(cases) {
		t.Errorf("schema has %d fields, want %d: %+v", len(schema), len(cases), schema)
	}
}

func TestDetectSchemaFromJSON_ReturnsStableAlphabeticalOrder(t *testing.T) {
	records := []map[string]interface{}{
		{"id": float64(1), "amount": float64(10), "createdAt": "2024-01-01"},
		{"id": float64(2), "amount": float64(20), "createdAt": "2024-01-02"},
	}

	// This is now a user-facing preview the user confirms, so the same
	// input must yield the same field order on every call — not whatever
	// order Go's randomized map iteration happens to produce this time.
	want := []string{"amount", "createdAt", "id"}

	for i := 0; i < 20; i++ {
		schema := detectSchemaFromJSON(records)
		if len(schema) != len(want) {
			t.Fatalf("run %d: schema has %d fields, want %d: %+v", i, len(schema), len(want), schema)
		}
		for j, f := range schema {
			if f.Name != want[j] {
				t.Fatalf("run %d: schema order = %v, want %v", i, fieldNames(schema), want)
			}
		}
	}
}

func fieldNames(schema []models.SchemaField) []string {
	names := make([]string, len(schema))
	for i, f := range schema {
		names[i] = f.Name
	}
	return names
}

// newIngestionServiceForTest builds an IngestionService with an unrestricted
// http.Client — DetectAPISchema's default client refuses loopback addresses
// (SSRF guard), which would also refuse httptest's own server, so tests that
// need a real HTTP round trip must swap it out.
func newIngestionServiceForTest() *IngestionService {
	return &IngestionService{httpClient: http.DefaultClient}
}

func TestDetectAPISchema_PushModeReturnsError(t *testing.T) {
	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePush,
		PullURL: "https://example.com/data",
	})

	if !errors.Is(err, ErrPushModeSchemaDetection) {
		t.Fatalf("expected ErrPushModeSchemaDetection, got %v", err)
	}
}

func TestDetectAPISchema_HappyPathWithRootPathAndSampleLimit(t *testing.T) {
	records := make([]map[string]interface{}, 0, 60)
	for i := 0; i < 60; i++ {
		records = append(records, map[string]interface{}{
			"id":        float64(i),
			"amount":    float64(i) * 10.5,
			"createdAt": "2024-03-01T00:00:00Z",
			"active":    i%2 == 0,
		})
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Api-Key") != "secret" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"data": map[string]interface{}{
				"records": records,
			},
		})
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	schema, sampleCount, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:               models.APIModePull,
		PullURL:            server.URL,
		PullMethod:         http.MethodGet,
		PullAuthType:       models.APIAuthAPIKey,
		PullAuthHeaderName: "X-Api-Key",
		PullAuthValue:      "secret",
		RootPath:           "data.records",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if sampleCount != maxSchemaDetectSampleRows {
		t.Errorf("sampleCount = %d, want %d (capped)", sampleCount, maxSchemaDetectSampleRows)
	}

	types := make(map[string]models.FieldType, len(schema))
	for _, f := range schema {
		types[f.Name] = f.Type
	}
	cases := map[string]models.FieldType{
		"id":        models.FieldNumber,
		"amount":    models.FieldNumber,
		"createdAt": models.FieldDate,
		"active":    models.FieldBoolean,
	}
	for field, want := range cases {
		got, ok := types[field]
		if !ok {
			t.Fatalf("field %q missing from detected schema: %+v", field, schema)
		}
		if got != want {
			t.Errorf("field %q: got type %q, want %q", field, got, want)
		}
	}
}

func TestDetectAPISchema_EmptyRecordsReturnsClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]interface{}{})
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for a response with no records, got nil")
	}
}

func TestDetectAPISchema_NonOKStatusReturnsClearError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte("invalid credentials"))
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for a non-2xx response, got nil")
	}
}

func TestDetectAPISchema_OversizedResponseReturnsSizeErrorNotJSONError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[`))
		// A body larger than maxPullResponseBytes must be reported as
		// oversized, not misdiagnosed as invalid JSON just because the read
		// was cut off mid-array.
		filler := make([]byte, maxPullResponseBytes+1024)
		for i := range filler {
			filler[i] = '0'
		}
		_, _ = w.Write(filler)
	}))
	defer server.Close()

	s := newIngestionServiceForTest()

	_, _, err := s.DetectAPISchema(context.Background(), models.SourceConfig{
		Mode:    models.APIModePull,
		PullURL: server.URL,
	})
	if err == nil {
		t.Fatal("expected an error for an oversized response, got nil")
	}
	if !strings.Contains(err.Error(), "supera el límite") {
		t.Fatalf("expected a size-limit error, got: %v", err)
	}
}

func schemaField(name string, t models.FieldType) models.SchemaField {
	return models.SchemaField{Name: name, Type: t}
}

func TestCompareSchema_ExactMatchSameOrder(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}

	diff := compareSchema(expected, actual)
	if !diff.Match {
		t.Errorf("esperaba match, obtuve %+v", diff)
	}
}

func TestCompareSchema_OrderIrrelevant(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("fecha", models.FieldDate)}
	actual := []models.SchemaField{schemaField("fecha", models.FieldDate), schemaField("monto", models.FieldNumber)}

	diff := compareSchema(expected, actual)
	if !diff.Match {
		t.Errorf("el orden de las columnas no debería importar, obtuve %+v", diff)
	}
}

func TestCompareSchema_MissingField(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("clavepais", models.FieldString)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por campo faltante")
	}
	if len(diff.MissingFields) != 1 || diff.MissingFields[0] != "clavepais" {
		t.Errorf("MissingFields = %v, esperaba [clavepais]", diff.MissingFields)
	}
}

func TestCompareSchema_ExtraField(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber)}
	actual := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("ticket", models.FieldString)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por campo de más")
	}
	if len(diff.ExtraFields) != 1 || diff.ExtraFields[0] != "ticket" {
		t.Errorf("ExtraFields = %v, esperaba [ticket]", diff.ExtraFields)
	}
}

func TestCompareSchema_TypeMismatch(t *testing.T) {
	expected := []models.SchemaField{schemaField("monto", models.FieldNumber)}
	actual := []models.SchemaField{schemaField("monto", models.FieldString)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("esperaba Match=false por tipo distinto")
	}
	if len(diff.TypeMismatches) != 1 || diff.TypeMismatches[0].Field != "monto" {
		t.Errorf("TypeMismatches = %+v, esperaba un mismatch en 'monto'", diff.TypeMismatches)
	}
	if diff.TypeMismatches[0].ExpectedType != models.FieldNumber || diff.TypeMismatches[0].ActualType != models.FieldString {
		t.Errorf("tipos del mismatch incorrectos: %+v", diff.TypeMismatches[0])
	}
}

func TestCompareSchema_EmptyExpectedAlwaysMatches(t *testing.T) {
	actual := []models.SchemaField{schemaField("cualquier_cosa", models.FieldString)}

	diff := compareSchema(nil, actual)
	if !diff.Match {
		t.Errorf("un monitor sin base establecida (primer upload) siempre debería matchear, obtuve %+v", diff)
	}
}

func TestCompareSchema_DateFieldWithFormatToleratesStringDetection(t *testing.T) {
	expected := []models.SchemaField{{Name: "fecha", Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}}
	actual := []models.SchemaField{schemaField("fecha", models.FieldString)}

	diff := compareSchema(expected, actual)
	if !diff.Match {
		t.Errorf("un campo date con DateFormat configurado debería tolerar la detección como string, obtuve %+v", diff)
	}
}

func TestCompareSchema_DateFieldWithoutFormatStillFlagsMismatch(t *testing.T) {
	expected := []models.SchemaField{{Name: "fecha", Type: models.FieldDate, DateFormat: ""}}
	actual := []models.SchemaField{schemaField("fecha", models.FieldString)}

	diff := compareSchema(expected, actual)
	if diff.Match {
		t.Fatal("un campo date SIN DateFormat configurado debe seguir marcando mismatch string/date")
	}
	if len(diff.TypeMismatches) != 1 || diff.TypeMismatches[0].Field != "fecha" {
		t.Errorf("TypeMismatches = %+v, esperaba un mismatch en 'fecha'", diff.TypeMismatches)
	}
}

func TestValidateFieldValue_NumberValid(t *testing.T) {
	if !validateFieldValue("1234.56", models.SchemaField{Type: models.FieldNumber}) {
		t.Error("esperaba que '1234.56' sea válido como number")
	}
}

func TestValidateFieldValue_NumberInvalid(t *testing.T) {
	if validateFieldValue("n/a", models.SchemaField{Type: models.FieldNumber}) {
		t.Error("esperaba que 'n/a' NO sea válido como number")
	}
}

func TestValidateFieldValue_DateValid(t *testing.T) {
	if !validateFieldValue("2026-09-08", models.SchemaField{Type: models.FieldDate}) {
		t.Error("esperaba que '2026-09-08' sea válido como date")
	}
}

func TestValidateFieldValue_DateInvalid(t *testing.T) {
	if validateFieldValue("no es una fecha", models.SchemaField{Type: models.FieldDate}) {
		t.Error("esperaba que 'no es una fecha' NO sea válido como date")
	}
}

func TestValidateFieldValue_DateFormatDDMMYYYY_Valid(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}
	if !validateFieldValue("08/09/2026", field) {
		t.Error("'08/09/2026' debería ser válido con DateFormat=DD/MM/YYYY")
	}
}

func TestValidateFieldValue_DateFormatConfigured_RejectsISO(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}
	if validateFieldValue("2026-09-08", field) {
		t.Error("con DateFormat configurado, un valor en formato ISO debe rechazarse (validación estricta, no aditiva)")
	}
}

func TestValidateFieldValue_NoDateFormat_StillAcceptsISO(t *testing.T) {
	field := models.SchemaField{Type: models.FieldDate}
	if !validateFieldValue("2026-09-08", field) {
		t.Error("sin DateFormat configurado, ISO sigue siendo válido (comportamiento actual sin cambios)")
	}
}

func TestValidateFieldValue_BooleanValid(t *testing.T) {
	if !validateFieldValue("true", models.SchemaField{Type: models.FieldBoolean}) {
		t.Error("esperaba que 'true' sea válido como boolean")
	}
}

func TestValidateFieldValue_BooleanInvalid(t *testing.T) {
	if validateFieldValue("tal vez", models.SchemaField{Type: models.FieldBoolean}) {
		t.Error("esperaba que 'tal vez' NO sea válido como boolean")
	}
}

func TestValidateFieldValue_StringAlwaysValid(t *testing.T) {
	if !validateFieldValue("cualquier texto 123", models.SchemaField{Type: models.FieldString}) {
		t.Error("un campo de tipo string siempre debería aceptar cualquier valor")
	}
}

func TestValidateFieldValue_EmptyAlwaysValid(t *testing.T) {
	if !validateFieldValue("", models.SchemaField{Type: models.FieldNumber}) {
		t.Error("una celda vacía no debería rechazarse por tipo — es un campo opcional sin valor, no un valor inválido")
	}
}

func TestFirstInvalidField_CleanRowReturnsEmpty(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	field, reason := firstInvalidField([]string{"100.50", "Juan"}, []string{"monto", "cliente"}, schema)
	if field != "" || reason != "" {
		t.Errorf("fila válida no debería rechazar nada, obtuve field=%q reason=%q", field, reason)
	}
}

func TestFirstInvalidField_BadTypeReturnsField(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	field, reason := firstInvalidField([]string{"no-es-numero", "Juan"}, []string{"monto", "cliente"}, schema)
	if field != "monto" {
		t.Errorf("field = %q, esperaba 'monto'", field)
	}
	if reason == "" {
		t.Error("esperaba una razón no vacía")
	}
}

func TestFirstInvalidField_MissingTrailingColumnsNotRejected(t *testing.T) {
	schema := []models.SchemaField{schemaField("monto", models.FieldNumber), schemaField("cliente", models.FieldString)}
	// Fila con menos columnas que headers — comportamiento ya tolerado hoy
	// por el resto del código (ver doc-building loop), no es un motivo de
	// rechazo por sí solo.
	field, reason := firstInvalidField([]string{"100.50"}, []string{"monto", "cliente"}, schema)
	if field != "" || reason != "" {
		t.Errorf("columnas faltantes al final no deberían rechazar la fila, obtuve field=%q reason=%q", field, reason)
	}
}

func TestParseValue_ImpliedDecimalsDividesCorrectly(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 5000.0 {
		t.Errorf("got %v, want 5000.0", f)
	}
}

func TestParseValue_ImpliedDecimalsZeroNoChange(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 0}}
	got := parseValue("500000", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 500000.0 {
		t.Errorf("got %v, want 500000.0 (sin cambios cuando ImpliedDecimals=0)", f)
	}
}

func TestParseValue_ImpliedDecimalsSmallValue(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("5", schema, "monto")
	f, ok := got.(float64)
	if !ok {
		t.Fatalf("esperaba float64, obtuve %T", got)
	}
	if f != 0.05 {
		t.Errorf("got %v, want 0.05", f)
	}
}

func TestParseValue_InvalidNumberFallsBackToRawString(t *testing.T) {
	schema := []models.SchemaField{{Name: "monto", Type: models.FieldNumber, ImpliedDecimals: 2}}
	got := parseValue("no-es-numero", schema, "monto")
	if got != "no-es-numero" {
		t.Errorf("got %v, want el string crudo sin parsear", got)
	}
}

func TestParseValue_DateFormatDDMMYYYY_ParsesCorrectLayout(t *testing.T) {
	schema := []models.SchemaField{{Name: "fecha", Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}}
	got := parseValue("08/09/2026", schema, "fecha")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("esperaba time.Time, obtuve %T (%v)", got, got)
	}
	if tm.Year() != 2026 || tm.Month() != time.September || tm.Day() != 8 {
		t.Errorf("got %v, want 8 de septiembre de 2026 (confirma que se usó DD/MM/YYYY, no MM/DD/YYYY)", tm)
	}
}

func TestParseValue_DateFormatDDMMYYYY_ParsesSingleDigitDayMonth(t *testing.T) {
	schema := []models.SchemaField{{Name: "fecha", Type: models.FieldDate, DateFormat: "DD/MM/YYYY"}}
	got := parseValue("8/9/2026", schema, "fecha")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("esperaba time.Time, obtuve %T (%v)", got, got)
	}
	if tm.Year() != 2026 || tm.Month() != time.September || tm.Day() != 8 {
		t.Errorf("got %v, want 8 de septiembre de 2026 (día/mes sin cero a la izquierda)", tm)
	}
}

func TestParseValue_NoDateFormat_StillParsesISO(t *testing.T) {
	schema := []models.SchemaField{{Name: "fecha", Type: models.FieldDate}}
	got := parseValue("2026-09-08", schema, "fecha")
	tm, ok := got.(time.Time)
	if !ok {
		t.Fatalf("esperaba time.Time, obtuve %T", got)
	}
	if tm.Year() != 2026 || tm.Month() != time.September || tm.Day() != 8 {
		t.Errorf("got %v, want 8 de septiembre de 2026 (regresión: sin DateFormat, ISO debe seguir parseando igual que hoy)", tm)
	}
}

func TestValidateFieldValue_ImpliedDecimalsRejectsDotInRawValue(t *testing.T) {
	field := models.SchemaField{Type: models.FieldNumber, ImpliedDecimals: 2}
	if validateFieldValue("500.5", field) {
		t.Error("un valor con punto decimal en un campo con ImpliedDecimals>0 debería ser inválido")
	}
}

func TestValidateFieldValue_NoImpliedDecimalsAcceptsDotInRawValue(t *testing.T) {
	field := models.SchemaField{Type: models.FieldNumber, ImpliedDecimals: 0}
	if !validateFieldValue("500.5", field) {
		t.Error("sin ImpliedDecimals configurado, un valor con punto decimal sigue siendo válido (comportamiento actual sin cambios)")
	}
}

func TestIsValidDateFormatPreset_EmptyIsValid(t *testing.T) {
	if !IsValidDateFormatPreset("") {
		t.Error("un DateFormat vacío (sin formato configurado) debe ser válido")
	}
}

func TestIsValidDateFormatPreset_KnownPresetsAreValid(t *testing.T) {
	for key := range DateFormatPresets {
		if !IsValidDateFormatPreset(key) {
			t.Errorf("%q está en DateFormatPresets y debería ser válido", key)
		}
	}
}

func TestIsValidDateFormatPreset_UnknownKeyIsInvalid(t *testing.T) {
	if IsValidDateFormatPreset("YYYY-DD-MM") {
		t.Error("una clave que no está en DateFormatPresets debe ser inválida")
	}
}

// El documento ingerido debe llevar el campo derivado como time.Time cuando
// el monitor lo tiene configurado, y no llevarlo cuando no.
func TestApplyDerivedTimestamp_AgregaCampoCuandoHayConfig(t *testing.T) {
	cfg := &models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}

	applyDerivedTimestamp(doc, cfg)

	got, ok := doc["timestamp"].(time.Time)
	if !ok {
		t.Fatalf("esperaba un time.Time en 'timestamp', got %T", doc["timestamp"])
	}
	if want := time.Date(2026, 9, 7, 2, 15, 17, 0, time.UTC); !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestApplyDerivedTimestamp_SinConfigNoTocaElDocumento(t *testing.T) {
	doc := bson.M{"fechapoliza": int32(20260907), "horapoliza": int32(21517)}
	applyDerivedTimestamp(doc, nil)
	if _, existe := doc["timestamp"]; existe {
		t.Error("sin configuración no debería agregarse ningún campo derivado")
	}
	if len(doc) != 2 {
		t.Errorf("el documento no debería cambiar, got %d campos", len(doc))
	}
}

// Una fila con hora ilegible se ingiere igual, sin el campo derivado: un
// timestamp inconstruible es dato incompleto, no dato inválido.
func TestApplyDerivedTimestamp_InconstruibleNoAgregaCampo(t *testing.T) {
	cfg := &models.DerivedTimestampConfig{
		DateField: "fechapoliza", DateFormat: "YYYYMMDD",
		TimeField: "horapoliza", TimeFormat: "HHMMSS",
		TargetName: "timestamp",
	}
	doc := bson.M{"fechapoliza": int32(20260907)}
	applyDerivedTimestamp(doc, cfg)
	if _, existe := doc["timestamp"]; existe {
		t.Error("no debería agregarse el campo si no se puede construir")
	}
}

// fakeUploadFile adapta un string a multipart.File (Reader + ReaderAt +
// Seeker + Closer) para poder ejercitar los caminos de ingesta sin subir
// un archivo real.
type fakeUploadFile struct{ *strings.Reader }

func (fakeUploadFile) Close() error { return nil }

func monitorConTimestampDerivado() *models.Monitor {
	return &models.Monitor{
		Name: "polizas",
		Schema: []models.SchemaField{
			{Name: "fechapoliza", Type: models.FieldNumber},
			{Name: "horapoliza", Type: models.FieldNumber},
			{Name: "tarjeta", Type: models.FieldString},
			// El campo derivado vive en el schema (para reglas, chat y
			// dashboards) pero nunca es una columna del archivo.
			{Name: "timestamp", Type: models.FieldDate},
		},
		DerivedTimestamp: &models.DerivedTimestampConfig{
			DateField: "fechapoliza", DateFormat: "YYYYMMDD",
			TimeField: "horapoliza", TimeFormat: "HHMMSS",
			TargetName: "timestamp",
		},
	}
}

// REGRESIÓN: configurar el timestamp derivado agregaba su campo al schema
// del monitor, y compareSchema lo reportaba como columna faltante en cada
// upload posterior — el monitor dejaba de ingerir para siempre (422).
func TestIngestCSV_ArchivoSinLaColumnaDerivadaSigueIngiriendo(t *testing.T) {
	monitor := monitorConTimestampDerivado()
	csv := "fechapoliza,horapoliza,tarjeta\n20260907,21517,T4111\n20260907,21520,T4111\n"

	svc := NewIngestionService(nil)
	outcome, err := svc.IngestCSV(context.Background(), monitor, fakeUploadFile{strings.NewReader(csv)}, true)
	if err != nil {
		t.Fatalf("IngestCSV devolvió error: %v", err)
	}
	if !outcome.Ingested {
		t.Errorf("el archivo debería ingerirse; diff = %+v", outcome.SchemaDiff)
	}
	// Ingested puede ser false por 'extra' o por 'mismatches' también: se
	// fija el síntoma exacto, que el campo derivado no cuente como faltante.
	for _, f := range outcome.SchemaDiff.MissingFields {
		if f == "timestamp" {
			t.Errorf("el campo derivado %q no debería participar de la comparación de estructura", f)
		}
	}
	if len(outcome.SchemaDiff.MissingFields) != 0 {
		t.Errorf("MissingFields = %v, want vacío", outcome.SchemaDiff.MissingFields)
	}
	if outcome.RowsAccepted != 2 {
		t.Errorf("RowsAccepted = %d, want 2", outcome.RowsAccepted)
	}
}

// El mismo camino, en Excel: compareSchema se llama igual en IngestExcel.
func TestIngestExcel_ArchivoSinLaColumnaDerivadaSigueIngiriendo(t *testing.T) {
	monitor := monitorConTimestampDerivado()
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()
	sheet := f.GetSheetName(0)
	filas := [][]interface{}{
		{"fechapoliza", "horapoliza", "tarjeta"},
		{20260907, 21517, "T4111"},
		{20260907, 21520, "T4111"},
	}
	for i, fila := range filas {
		cell, err := excelize.CoordinatesToCellName(1, i+1)
		if err != nil {
			t.Fatalf("CoordinatesToCellName: %v", err)
		}
		if err := f.SetSheetRow(sheet, cell, &fila); err != nil {
			t.Fatalf("SetSheetRow: %v", err)
		}
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("escribiendo el xlsx: %v", err)
	}

	svc := NewIngestionService(nil)
	outcome, err := svc.IngestExcel(context.Background(), monitor, fakeUploadFile{strings.NewReader(buf.String())}, true)
	if err != nil {
		t.Fatalf("IngestExcel devolvió error: %v", err)
	}
	if !outcome.Ingested {
		t.Errorf("el archivo debería ingerirse; diff = %+v", outcome.SchemaDiff)
	}
	if len(outcome.SchemaDiff.MissingFields) != 0 {
		t.Errorf("MissingFields = %v, want vacío", outcome.SchemaDiff.MissingFields)
	}
}

func TestSchemaForComparison_ExcluyeSoloElCampoDerivado(t *testing.T) {
	monitor := monitorConTimestampDerivado()
	got := schemaForComparison(monitor.Schema, monitor.DerivedTimestamp)
	if len(got) != 3 {
		t.Fatalf("got %d campos, want 3: %+v", len(got), got)
	}
	for _, f := range got {
		if f.Name == "timestamp" {
			t.Error("el campo derivado no debería estar en el schema de comparación")
		}
	}
	// El schema original no se toca: parseValue y firstInvalidField siguen
	// necesitando saber que el campo derivado es de tipo date.
	if len(monitor.Schema) != 4 {
		t.Errorf("el schema del monitor fue mutado: %+v", monitor.Schema)
	}
}

func TestSchemaForComparison_SinConfigDevuelveElMismoSchema(t *testing.T) {
	schema := []models.SchemaField{{Name: "importe", Type: models.FieldNumber}}
	got := schemaForComparison(schema, nil)
	if len(got) != 1 || got[0].Name != "importe" {
		t.Errorf("sin configuración el schema debe pasar intacto, got %+v", got)
	}
}

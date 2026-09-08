# Bitácora de Cargas Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Validar la estructura de cada archivo subido a un monitor contra su base establecida antes de ingerirlo (rechazando el archivo completo si no coincide, o filas puntuales si fallan por tipo), con verificación previa automática al hacer click en "Subir" y una bitácora persistida y filtrable de cada intento real de carga.

**Architecture:** Una función pura `compareSchema` (nombres + tipos, orden irrelevante) decide si un archivo coincide con la base del monitor; una segunda función pura valida cada fila por tipo. Ambas se integran directamente en los parsers existentes de `ingestion_service.go` (CSV/TXT/Excel comparten forma de fila y ganan la validación; JSON queda fuera de esta iteración, ver spec). Un nuevo repo (`UploadLogRepository`) persiste metadata de cada intento en Mongo y, cuando corresponde, el archivo rechazado en GridFS (mismo patrón que `RedFlagReportRepository`) para permitir aprobación y re-ingesta posterior. El frontend llama un endpoint de verificación (`dryRun`) automáticamente al soltar el archivo en el dropzone existente, antes de subir de verdad.

**Tech Stack:** Go/Fiber/MongoDB (backend), Next.js/React/TypeScript/Zustand (frontend) — mismo stack que el resto del proyecto, sin dependencias nuevas.

**Spec:** `docs/superpowers/specs/2026-09-08-bitacora-cargas-design.md`

## Global Constraints

- La comparación de estructura es fija: mismos nombres de campo (sin importar orden) + mismo tipo inferido por campo. No hay tolerancia configurable por monitor.
- La validación por fila cubre CSV, TXT y Excel (comparten forma de fila `[]string`) — JSON queda fuera de esta iteración (ver spec, sección "Fuera de alcance").
- Los monitores ya existentes con datos mezclados (ej. "CBCG Tarjetas") NO se tocan — la validación aplica solo hacia adelante.
- Ningún componente frontend escribe un color literal — usar `STATUS_CLASSES`/`CATEGORY_CLASSES` de `@/lib/semantic-colors`, igual que el resto del proyecto (`CLAUDE.md`).
- Aprobar una entrada rechazada está restringido a roles `admin`/`compliance` (`RequireComplianceOrAbove()` en backend, `user.role !== "viewer"` en frontend) — mismo criterio que ya protege la subida.
- Sin tests de componentes React (no hay convención de testing de componentes en este proyecto). Sin tests HTTP de handlers más allá de funciones puras extraídas (convención confirmada en `dashboard_handler_test.go`).

---

### Task 1: Modelo de datos de la bitácora + `compareSchema`

**Files:**
- Create: `backend/internal/models/upload_log.go`
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Produces: `models.UploadStatus`, `models.FieldTypeMismatch`, `models.SchemaDiff`, `models.RowRejection`, `models.UploadLogEntry` (Tareas 3-6 los consumen). `compareSchema(expected, actual []models.SchemaField) models.SchemaDiff` (Tareas 2, 4, 5 lo consumen).

- [ ] **Step 1: Crear el modelo de datos**

Crear `backend/internal/models/upload_log.go`:

```go
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type UploadStatus string

const (
	UploadStatusAccepted          UploadStatus = "accepted"
	UploadStatusPartial           UploadStatus = "partial"
	UploadStatusRejectedStructure UploadStatus = "rejected_structure"
	UploadStatusApproved          UploadStatus = "approved"
)

// FieldTypeMismatch describes one field whose type in the uploaded file
// doesn't match the monitor's established schema.
type FieldTypeMismatch struct {
	Field        string    `bson:"field" json:"field"`
	ExpectedType FieldType `bson:"expected_type" json:"expectedType"`
	ActualType   FieldType `bson:"actual_type" json:"actualType"`
}

// SchemaDiff is the result of comparing a file's detected structure
// against a monitor's established schema. Match is true only when
// MissingFields, ExtraFields and TypeMismatches are all empty.
type SchemaDiff struct {
	Match          bool                `bson:"match" json:"match"`
	MissingFields  []string            `bson:"missing_fields,omitempty" json:"missingFields,omitempty"`
	ExtraFields    []string            `bson:"extra_fields,omitempty" json:"extraFields,omitempty"`
	TypeMismatches []FieldTypeMismatch `bson:"type_mismatches,omitempty" json:"typeMismatches,omitempty"`
}

// RowRejection records one row that was skipped during ingestion because
// a field's value didn't parse as its schema type. RowIndex is 1-based,
// counting only data rows (the header row, if any, is not row 1).
type RowRejection struct {
	RowIndex int    `bson:"row_index" json:"rowIndex"`
	Field    string `bson:"field" json:"field"`
	Reason   string `bson:"reason" json:"reason"`
}

// UploadLogEntry is one row of the upload audit log — one per real
// upload attempt (not per dry-run check, those are never persisted).
type UploadLogEntry struct {
	ID              primitive.ObjectID  `bson:"_id,omitempty" json:"id"`
	MonitorID       primitive.ObjectID  `bson:"monitor_id" json:"monitorId"`
	MonitorName     string              `bson:"monitor_name" json:"monitorName"`
	FileName        string              `bson:"file_name" json:"fileName"`
	SourceType      SourceType          `bson:"source_type" json:"sourceType"`
	UploadedBy      primitive.ObjectID  `bson:"uploaded_by" json:"uploadedBy"`
	UploadedByEmail string              `bson:"uploaded_by_email" json:"uploadedByEmail"`
	UploadedAt      time.Time           `bson:"uploaded_at" json:"uploadedAt"`
	Status          UploadStatus        `bson:"status" json:"status"`
	TotalRows       int                 `bson:"total_rows" json:"totalRows"`
	RowsAccepted    int                 `bson:"rows_accepted" json:"rowsAccepted"`
	RowsRejected    int                 `bson:"rows_rejected" json:"rowsRejected"`
	SchemaDiff      *SchemaDiff         `bson:"schema_diff,omitempty" json:"schemaDiff,omitempty"`
	RowRejections   []RowRejection      `bson:"row_rejections,omitempty" json:"rowRejections,omitempty"`
	SampleRows      []map[string]any    `bson:"sample_rows,omitempty" json:"sampleRows,omitempty"`
	GridFSFileID    *primitive.ObjectID `bson:"gridfs_file_id,omitempty" json:"-"`
}
```

- [ ] **Step 2: Escribir los tests de `compareSchema` (deben fallar — la función no existe)**

Agregar al final de `backend/internal/services/ingestion_service_test.go`:

```go
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
```

- [ ] **Step 3: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run TestCompareSchema -v`
Expected: FAIL — `undefined: compareSchema`

- [ ] **Step 4: Implementar `compareSchema`**

Agregar a `backend/internal/services/ingestion_service.go` (agregar `"sort"` al bloque de imports existente):

```go
// compareSchema compares a newly-detected file schema against a
// monitor's established schema. Field order doesn't matter — only the
// set of names and, per matching name, the type. An empty expected
// schema (the monitor's first upload, nothing to compare against yet)
// always matches.
func compareSchema(expected []models.SchemaField, actual []models.SchemaField) models.SchemaDiff {
	if len(expected) == 0 {
		return models.SchemaDiff{Match: true}
	}

	expectedByName := make(map[string]models.SchemaField, len(expected))
	for _, f := range expected {
		expectedByName[f.Name] = f
	}
	actualByName := make(map[string]models.SchemaField, len(actual))
	for _, f := range actual {
		actualByName[f.Name] = f
	}

	var missing, extra []string
	var mismatches []models.FieldTypeMismatch

	for name, exp := range expectedByName {
		act, ok := actualByName[name]
		if !ok {
			missing = append(missing, name)
			continue
		}
		if act.Type != exp.Type {
			mismatches = append(mismatches, models.FieldTypeMismatch{
				Field:        name,
				ExpectedType: exp.Type,
				ActualType:   act.Type,
			})
		}
	}
	for name := range actualByName {
		if _, ok := expectedByName[name]; !ok {
			extra = append(extra, name)
		}
	}

	sort.Strings(missing)
	sort.Strings(extra)
	sort.Slice(mismatches, func(i, j int) bool { return mismatches[i].Field < mismatches[j].Field })

	return models.SchemaDiff{
		Match:          len(missing) == 0 && len(extra) == 0 && len(mismatches) == 0,
		MissingFields:  missing,
		ExtraFields:    extra,
		TypeMismatches: mismatches,
	}
}
```

- [ ] **Step 5: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run TestCompareSchema -v`
Expected: PASS (6/6)

- [ ] **Step 6: Commit**

```bash
git add backend/internal/models/upload_log.go backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(bitacora): modelo de datos y compareSchema"
```

---

### Task 2: Validación por fila

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`
- Test: `backend/internal/services/ingestion_service_test.go`

**Interfaces:**
- Consumes: `models.SchemaField`, `models.FieldType` (existentes), `models.RowRejection` (Tarea 1).
- Produces: `validateFieldValue(raw string, expected models.FieldType) bool`, `firstInvalidField(row []string, fieldNames []string, schema []models.SchemaField) (field string, reason string)` — Tarea 4 los consume.

- [ ] **Step 1: Escribir los tests (deben fallar)**

Agregar al final de `backend/internal/services/ingestion_service_test.go`:

```go
func TestValidateFieldValue_NumberValid(t *testing.T) {
	if !validateFieldValue("1234.56", models.FieldNumber) {
		t.Error("esperaba que '1234.56' sea válido como number")
	}
}

func TestValidateFieldValue_NumberInvalid(t *testing.T) {
	if validateFieldValue("n/a", models.FieldNumber) {
		t.Error("esperaba que 'n/a' NO sea válido como number")
	}
}

func TestValidateFieldValue_DateValid(t *testing.T) {
	if !validateFieldValue("2026-09-08", models.FieldDate) {
		t.Error("esperaba que '2026-09-08' sea válido como date")
	}
}

func TestValidateFieldValue_DateInvalid(t *testing.T) {
	if validateFieldValue("no es una fecha", models.FieldDate) {
		t.Error("esperaba que 'no es una fecha' NO sea válido como date")
	}
}

func TestValidateFieldValue_BooleanValid(t *testing.T) {
	if !validateFieldValue("true", models.FieldBoolean) {
		t.Error("esperaba que 'true' sea válido como boolean")
	}
}

func TestValidateFieldValue_BooleanInvalid(t *testing.T) {
	if validateFieldValue("tal vez", models.FieldBoolean) {
		t.Error("esperaba que 'tal vez' NO sea válido como boolean")
	}
}

func TestValidateFieldValue_StringAlwaysValid(t *testing.T) {
	if !validateFieldValue("cualquier texto 123", models.FieldString) {
		t.Error("un campo de tipo string siempre debería aceptar cualquier valor")
	}
}

func TestValidateFieldValue_EmptyAlwaysValid(t *testing.T) {
	if !validateFieldValue("", models.FieldNumber) {
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
```

- [ ] **Step 2: Correr los tests, confirmar que fallan**

Run: `cd backend && go test ./internal/services/... -run "TestValidateFieldValue|TestFirstInvalidField" -v`
Expected: FAIL — `undefined: validateFieldValue` / `undefined: firstInvalidField`

- [ ] **Step 3: Implementar ambas funciones**

Agregar a `backend/internal/services/ingestion_service.go`, cerca de `parseValue`:

```go
// validateFieldValue reports whether raw parses cleanly as expected —
// mirrors the parsing branches of parseValue, but reports failure
// instead of silently falling back to the raw string. An empty value
// is always valid (an unset optional cell, not a bad value).
func validateFieldValue(raw string, expected models.FieldType) bool {
	if raw == "" {
		return true
	}
	switch expected {
	case models.FieldNumber:
		_, err := strconv.ParseFloat(raw, 64)
		return err == nil
	case models.FieldBoolean:
		_, err := strconv.ParseBool(raw)
		return err == nil
	case models.FieldDate:
		if _, err := time.Parse("2006-01-02", raw); err == nil {
			return true
		}
		_, err := time.Parse(time.RFC3339, raw)
		return err == nil
	default:
		return true
	}
}

// firstInvalidField returns the first field in fieldNames whose raw
// string value in row doesn't parse as its schema type, or ("", "") if
// the row is clean. A row shorter than fieldNames (missing trailing
// columns) is not rejected here — that's tolerated today by the
// doc-building loop in ingestDelimited/IngestExcel, and stays that way;
// this only rejects values that ARE present but don't parse.
func firstInvalidField(row []string, fieldNames []string, schema []models.SchemaField) (field string, reason string) {
	for i, name := range fieldNames {
		if i >= len(row) {
			continue
		}
		for _, f := range schema {
			if f.Name == name {
				if !validateFieldValue(row[i], f.Type) {
					return name, fmt.Sprintf("valor %q no es del tipo %s", row[i], f.Type)
				}
				break
			}
		}
	}
	return "", ""
}
```

- [ ] **Step 4: Correr los tests, confirmar que pasan**

Run: `cd backend && go test ./internal/services/... -run "TestValidateFieldValue|TestFirstInvalidField" -v`
Expected: PASS (10/10)

- [ ] **Step 5: Commit**

```bash
git add backend/internal/services/ingestion_service.go backend/internal/services/ingestion_service_test.go
git commit -m "feat(bitacora): validación de valores por fila"
```

---

### Task 3: `UploadLogRepository`

**Files:**
- Create: `backend/internal/repository/upload_log_repo.go`

**Interfaces:**
- Consumes: `models.UploadLogEntry`, `models.UploadStatus` (Tarea 1).
- Produces: `UploadLogRepository` con `Save`, `List`, `Get`, `DownloadRejectedFile`, `MarkApproved` — Tareas 4-6 lo consumen.

Sin tests — mismo criterio que `RedFlagReportRepository` (ningún repositorio de este proyecto tiene archivo de test propio, confirmado: no existe `backend/internal/repository/*_test.go`).

- [ ] **Step 1: Crear el repositorio**

Crear `backend/internal/repository/upload_log_repo.go`:

```go
package repository

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// UploadLogRepository guarda la bitácora de cargas de archivos: metadata
// de cada intento (accepted/partial/rejected_structure/approved) en una
// colección, y el binario del archivo rechazado (pendiente de
// aprobación) en GridFS — mismo patrón que RedFlagReportRepository, sin
// índice único: un monitor tiene muchos intentos de carga, no uno por
// entidad.
type UploadLogRepository struct {
	col    *mongo.Collection
	bucket *gridfs.Bucket
}

func NewUploadLogRepository(db *database.MongoDB) *UploadLogRepository {
	col := db.Collection("upload_log")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := col.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{Keys: bson.D{{Key: "monitor_id", Value: 1}, {Key: "uploaded_at", Value: -1}}},
		{Keys: bson.D{{Key: "status", Value: 1}}},
	}); err != nil {
		fmt.Printf("WARNING: failed to create upload_log indexes: %v\n", err)
	}

	bucket, err := gridfs.NewBucket(db.Database, options.GridFSBucket().SetName("upload_rejections_files"))
	if err != nil {
		// No tumba el arranque del servidor — igual que red_flag_report_repo.go:
		// la aprobación con re-ingesta automática queda inoperante, cada
		// intento de usarla lo loggea (ver Save/DownloadRejectedFile).
		fmt.Printf("WARNING: failed to open upload_rejections_files GridFS bucket: %v\n", err)
		bucket = nil
	}

	return &UploadLogRepository{col: col, bucket: bucket}
}

// Save inserta una nueva entrada de bitácora. Si fileBytes no es nil (un
// rechazo de estructura que podría aprobarse y re-ingerirse después), se
// sube primero a GridFS y entry.GridFSFileID queda seteado antes del
// insert. Si el bucket no está disponible, la entrada se guarda igual,
// sin archivo — la aprobación posterior pedirá volver a subirlo.
func (r *UploadLogRepository) Save(ctx context.Context, entry *models.UploadLogEntry, fileBytes []byte) error {
	if fileBytes != nil {
		if r.bucket == nil {
			fmt.Printf("WARNING: upload_rejections_files bucket no disponible, entrada guardada sin archivo: %s\n", entry.FileName)
		} else {
			fileID, err := r.bucket.UploadFromStream(entry.FileName, bytes.NewReader(fileBytes))
			if err != nil {
				return fmt.Errorf("subiendo archivo rechazado a gridfs: %w", err)
			}
			entry.GridFSFileID = &fileID
		}
	}
	entry.ID = primitive.NewObjectID()
	if _, err := r.col.InsertOne(ctx, entry); err != nil {
		return fmt.Errorf("guardando entrada de bitácora: %w", err)
	}
	return nil
}

// List devuelve entradas de la bitácora, opcionalmente filtradas por
// monitor, más recientes primero.
func (r *UploadLogRepository) List(ctx context.Context, monitorID *primitive.ObjectID) ([]models.UploadLogEntry, error) {
	filter := bson.M{}
	if monitorID != nil {
		filter["monitor_id"] = *monitorID
	}
	opts := options.Find().SetSort(bson.D{{Key: "uploaded_at", Value: -1}})
	cursor, err := r.col.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("listando bitácora: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	entries := []models.UploadLogEntry{}
	if err := cursor.All(ctx, &entries); err != nil {
		return nil, fmt.Errorf("decodificando bitácora: %w", err)
	}
	return entries, nil
}

// Get busca una entrada por id. Devuelve mongo.ErrNoDocuments si no existe.
func (r *UploadLogRepository) Get(ctx context.Context, id primitive.ObjectID) (*models.UploadLogEntry, error) {
	var entry models.UploadLogEntry
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&entry); err != nil {
		return nil, err
	}
	return &entry, nil
}

// DownloadRejectedFile descarga el archivo guardado de una entrada
// rejected_structure. Error explícito si la entrada no tiene archivo
// guardado (bucket no disponible al momento del rechazo).
func (r *UploadLogRepository) DownloadRejectedFile(ctx context.Context, entry *models.UploadLogEntry) ([]byte, error) {
	if entry.GridFSFileID == nil {
		return nil, fmt.Errorf("no hay archivo guardado para esta entrada, hay que volver a subirlo")
	}
	if r.bucket == nil {
		return nil, fmt.Errorf("gridfs bucket no disponible")
	}
	var buf bytes.Buffer
	if _, err := r.bucket.DownloadToStream(*entry.GridFSFileID, &buf); err != nil {
		return nil, fmt.Errorf("descargando archivo rechazado de gridfs: %w", err)
	}
	return buf.Bytes(), nil
}

// MarkApproved actualiza una entrada a estado "approved" con el
// resultado real de la re-ingesta, y borra su archivo de GridFS (ya se
// re-ingirió, no debe quedar huérfano).
func (r *UploadLogRepository) MarkApproved(ctx context.Context, id primitive.ObjectID, totalRows, rowsAccepted, rowsRejected int, rowRejections []models.RowRejection) error {
	entry, err := r.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("buscando entrada a aprobar: %w", err)
	}
	if entry.GridFSFileID != nil && r.bucket != nil {
		_ = r.bucket.Delete(*entry.GridFSFileID)
	}
	status := models.UploadStatusApproved
	_, err = r.col.UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{
		"status":         status,
		"total_rows":     totalRows,
		"rows_accepted":  rowsAccepted,
		"rows_rejected":  rowsRejected,
		"row_rejections": rowRejections,
		"gridfs_file_id": nil,
	}})
	if err != nil {
		return fmt.Errorf("actualizando entrada aprobada: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 3: Commit**

```bash
git add backend/internal/repository/upload_log_repo.go
git commit -m "feat(bitacora): repositorio de la bitácora (metadata + GridFS)"
```

---

### Task 4: Integrar validación en la ingesta (CSV/TXT/Excel) + wiring en `UploadData`

**Files:**
- Modify: `backend/internal/services/ingestion_service.go`
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/cmd/server/main.go`

**Interfaces:**
- Consumes: `compareSchema`, `validateFieldValue`, `firstInvalidField` (Tareas 1-2); `UploadLogRepository` (Tarea 3).
- Produces: `IngestOutcome` (struct), `ingestDelimited`/`IngestCSV`/`IngestTXT`/`IngestExcel` con firma nueva `(ctx, monitor, file, dryRun bool) (*IngestOutcome, error)` — Tarea 5 (dry-run) y Tarea 6 (aprobación) lo consumen.

Sin TDD nuevo en este task — la lógica pura ya está testeada (Tareas 1-2); esto es integración mecánica. Se verifica corriendo los tests existentes (deben seguir pasando) más una verificación manual.

- [ ] **Step 1: Agregar el tipo `IngestOutcome`**

Agregar a `backend/internal/services/ingestion_service.go`, cerca de la definición de `IngestionService`:

```go
// IngestOutcome carries the result of a validated ingestion attempt for
// CSV/TXT/Excel (the three formats sharing the raw-string-row shape).
// Ingested is false only when the file's overall structure didn't match
// the monitor's established schema (SchemaDiff.Match == false) — in
// that case nothing was written to Mongo. When dryRun is true, Ingested
// reflects what WOULD have happened, but nothing is written either way.
type IngestOutcome struct {
	Ingested       bool
	DetectedSchema []models.SchemaField
	SchemaDiff     models.SchemaDiff
	TotalRows      int
	RowsAccepted   int
	RowRejections  []models.RowRejection
}
```

- [ ] **Step 2: Reescribir `ingestDelimited` con comparación de estructura, validación por fila, y modo `dryRun`**

Reemplazar la función completa `ingestDelimited` (la que empieza `func (s *IngestionService) ingestDelimited(ctx context.Context, monitor *models.Monitor, file multipart.File, defaultDelimiter rune) (int, error) {` y termina justo antes de `// IngestJSON parses...`) por:

```go
func (s *IngestionService) ingestDelimited(ctx context.Context, monitor *models.Monitor, file multipart.File, defaultDelimiter rune, dryRun bool) (*IngestOutcome, error) {
	delimiter := defaultDelimiter
	hasHeaderRow := true
	if monitor.SourceConfig != nil {
		if monitor.SourceConfig.Delimiter != "" {
			// DecodeRuneInString, not Delimiter[0]: indexing a Go string
			// takes the first byte, not the first UTF-8 rune. A multibyte
			// delimiter like "§" would silently become an unrelated rune
			// that never matches anything in the file, so every line would
			// parse as one column with no error raised.
			r, size := utf8.DecodeRuneInString(monitor.SourceConfig.Delimiter)
			if r != utf8.RuneError || size != 0 {
				delimiter = r
			}
		}
		if monitor.SourceConfig.HasHeaderRow != nil {
			hasHeaderRow = *monitor.SourceConfig.HasHeaderRow
		}
	}

	reader := csv.NewReader(file)
	reader.Comma = delimiter

	var headers []string
	var allRows [][]string

	if hasHeaderRow {
		h, err := reader.Read()
		if err != nil {
			return nil, fmt.Errorf("reading headers: %w", err)
		}
		headers = h
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w", err)
		}
		if headers == nil {
			// No header row: synthesize column names from the first row's width.
			headers = make([]string, len(row))
			for i := range row {
				headers[i] = fmt.Sprintf("col_%d", i+1)
			}
		}
		allRows = append(allRows, row)
	}

	// Real exports routinely have blank or repeated header cells (unlabeled
	// trailing columns, copy-pasted headers). Without deduping, two columns
	// sanitizing to the same field name collide when written into the same
	// BSON document — the second silently overwrites the first, so a monitor
	// can report N detected columns while actually storing fewer.
	fieldNames := dedupeFieldNames(headers)
	detectedSchema := detectSchemaFromCSV(fieldNames, allRows)

	diff := compareSchema(monitor.Schema, detectedSchema)
	if !diff.Match {
		return &IngestOutcome{
			Ingested:       false,
			DetectedSchema: detectedSchema,
			SchemaDiff:     diff,
			TotalRows:      len(allRows),
		}, nil
	}

	schema := monitor.Schema
	if len(schema) == 0 {
		schema = detectedSchema
	}

	documents := make([]interface{}, 0, len(allRows))
	var rejections []models.RowRejection
	for i, row := range allRows {
		if field, reason := firstInvalidField(row, fieldNames, schema); field != "" {
			rejections = append(rejections, models.RowRejection{RowIndex: i + 1, Field: field, Reason: reason})
			continue
		}
		doc := bson.M{"_ingested_at": time.Now()}
		for j, name := range fieldNames {
			if j < len(row) {
				doc[name] = parseValue(row[j], schema, name)
			}
		}
		documents = append(documents, doc)
	}

	outcome := &IngestOutcome{
		Ingested:       true,
		DetectedSchema: detectedSchema,
		SchemaDiff:     diff,
		TotalRows:      len(allRows),
		RowsAccepted:   len(documents),
		RowRejections:  rejections,
	}

	if dryRun {
		return outcome, nil
	}

	monitor.Schema = schema
	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return nil, fmt.Errorf("inserting data: %w", err)
	}
	outcome.RowsAccepted = count

	now := time.Now()
	if err := s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	}); err != nil {
		log.Printf("ingestion: actualizando estado del monitor: %v", err)
	}

	return outcome, nil
}
```

- [ ] **Step 3: Actualizar `IngestCSV`/`IngestTXT`**

Reemplazar:
```go
func (s *IngestionService) IngestCSV(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, ',')
}

func (s *IngestionService) IngestTXT(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, '\t')
}
```
por:
```go
func (s *IngestionService) IngestCSV(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	return s.ingestDelimited(ctx, monitor, file, ',', dryRun)
}

func (s *IngestionService) IngestTXT(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	return s.ingestDelimited(ctx, monitor, file, '\t', dryRun)
}
```

- [ ] **Step 4: Reescribir `IngestExcel` con el mismo tratamiento**

Reemplazar la función completa `IngestExcel` por:

```go
func (s *IngestionService) IngestExcel(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("opening Excel: %w", err)
	}
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)
	if monitor.SourceConfig != nil && monitor.SourceConfig.SheetName != "" {
		sheetName = monitor.SourceConfig.SheetName
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("reading Excel rows: %w", err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("excel file has no data rows")
	}

	headers := rows[0]
	dataRows := rows[1:]

	fieldNames := dedupeFieldNames(headers)
	detectedSchema := detectSchemaFromCSV(fieldNames, dataRows)

	diff := compareSchema(monitor.Schema, detectedSchema)
	if !diff.Match {
		return &IngestOutcome{
			Ingested:       false,
			DetectedSchema: detectedSchema,
			SchemaDiff:     diff,
			TotalRows:      len(dataRows),
		}, nil
	}

	schema := monitor.Schema
	if len(schema) == 0 {
		schema = detectedSchema
	}

	documents := make([]interface{}, 0, len(dataRows))
	var rejections []models.RowRejection
	for i, row := range dataRows {
		if field, reason := firstInvalidField(row, fieldNames, schema); field != "" {
			rejections = append(rejections, models.RowRejection{RowIndex: i + 1, Field: field, Reason: reason})
			continue
		}
		doc := bson.M{"_ingested_at": time.Now()}
		for j, name := range fieldNames {
			if j < len(row) {
				doc[name] = parseValue(row[j], schema, name)
			}
		}
		documents = append(documents, doc)
	}

	outcome := &IngestOutcome{
		Ingested:       true,
		DetectedSchema: detectedSchema,
		SchemaDiff:     diff,
		TotalRows:      len(dataRows),
		RowsAccepted:   len(documents),
		RowRejections:  rejections,
	}

	if dryRun {
		return outcome, nil
	}

	monitor.Schema = schema
	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return nil, fmt.Errorf("inserting data: %w", err)
	}
	outcome.RowsAccepted = count

	now := time.Now()
	if err := s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	}); err != nil {
		log.Printf("ingestion: actualizando estado del monitor: %v", err)
	}

	return outcome, nil
}
```

- [ ] **Step 5: Actualizar `MonitorHandler` — agregar dependencia de `UploadLogRepository`**

En `backend/internal/handlers/monitor_handler.go`, reemplazar el struct y constructor:

```go
type MonitorHandler struct {
	monitorRepo      *repository.MonitorRepository
	ingestionService *services.IngestionService
	jobQueue         *services.JobQueue
	ruleEngine       *services.RuleEngine
	uploadLogRepo    *repository.UploadLogRepository
}

func NewMonitorHandler(
	monitorRepo *repository.MonitorRepository,
	ingestionService *services.IngestionService,
	jobQueue *services.JobQueue,
	ruleEngine *services.RuleEngine,
	uploadLogRepo *repository.UploadLogRepository,
) *MonitorHandler {
	return &MonitorHandler{
		monitorRepo:      monitorRepo,
		ingestionService: ingestionService,
		jobQueue:         jobQueue,
		ruleEngine:       ruleEngine,
		uploadLogRepo:    uploadLogRepo,
	}
}
```

- [ ] **Step 6: Reescribir `UploadData` para usar la validación y guardar en la bitácora**

Reemplazar la función completa `UploadData` (`monitor_handler.go:304-356` aprox.) por:

```go
func (h *MonitorHandler) UploadData(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "opening file"})
	}
	defer func() { _ = f.Close() }()

	// El archivo se lee una sola vez desde el multipart; si el intento se
	// rechaza por estructura, se sube a GridFS desde estos mismos bytes
	// (leídos antes de que Ingest* consuma el reader).
	rawBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "reading file"})
	}
	reReadable := &memFile{Reader: bytes.NewReader(rawBytes)}

	var outcome *services.IngestOutcome
	switch monitor.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, reReadable, false)
	case models.SourceJSON:
		count, jerr := h.ingestionService.IngestJSON(c.Context(), monitor, bytes.NewReader(rawBytes))
		if jerr == nil {
			outcome = &services.IngestOutcome{Ingested: true, TotalRows: count, RowsAccepted: count}
		}
		err = jerr
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, reReadable, false)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, reReadable, false)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "unsupported source type"})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	userID, _ := primitive.ObjectIDFromHex(c.Locals("userId").(string))
	userEmail, _ := c.Locals("email").(string)

	entry := &models.UploadLogEntry{
		MonitorID:       id,
		MonitorName:     monitor.Name,
		FileName:        fileHeader.Filename,
		SourceType:      monitor.SourceType,
		UploadedBy:      userID,
		UploadedByEmail: userEmail,
		UploadedAt:      time.Now(),
		TotalRows:       outcome.TotalRows,
	}

	if !outcome.Ingested {
		entry.Status = models.UploadStatusRejectedStructure
		entry.SchemaDiff = &outcome.SchemaDiff
		entry.RowsRejected = outcome.TotalRows
		if saveErr := h.uploadLogRepo.Save(c.Context(), entry, rawBytes); saveErr != nil {
			log.Printf("bitacora: guardando entrada rechazada: %v", saveErr)
		}
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{
			"error":      "el archivo no coincide con la estructura del monitor",
			"schemaDiff": outcome.SchemaDiff,
		})
	}

	entry.RowsAccepted = outcome.RowsAccepted
	entry.RowsRejected = len(outcome.RowRejections)
	entry.RowRejections = outcome.RowRejections
	if entry.RowsRejected > 0 {
		entry.Status = models.UploadStatusPartial
	} else {
		entry.Status = models.UploadStatusAccepted
	}
	if saveErr := h.uploadLogRepo.Save(c.Context(), entry, nil); saveErr != nil {
		log.Printf("bitacora: guardando entrada aceptada: %v", saveErr)
	}

	// Enqueue rule evaluation (async via worker)
	queued := false
	if h.jobQueue != nil {
		qerr := h.jobQueue.Enqueue(c.Context(), services.EvalJob{MonitorID: id.Hex()})
		queued = qerr == nil
	}

	return c.JSON(fiber.Map{
		"recordsIngested":  outcome.RowsAccepted,
		"rowsRejected":     entry.RowsRejected,
		"schema":           monitor.Schema,
		"evaluationQueued": queued,
	})
}

// memFile adapta un *bytes.Reader a multipart.File (io.Reader +
// io.ReaderAt + io.Seeker + io.Closer) para poder re-parsear el mismo
// archivo dos veces (una para detectar, otra si Ingest* lo necesita) sin
// volver a leer del stream multipart original.
type memFile struct {
	*bytes.Reader
}

func (memFile) Close() error { return nil }
```

El bloque de imports de `monitor_handler.go` ya tiene `"bytes"` — agregarle `"io"`, `"log"` y `"time"` (ninguno de los tres está importado todavía).

- [ ] **Step 7: Actualizar `main.go`**

En `backend/cmd/server/main.go`, agregar cerca de `redFlagReportRepo := repository.NewRedFlagReportRepository(mongo)`:

```go
uploadLogRepo := repository.NewUploadLogRepository(mongo)
```

Y actualizar la línea de `NewMonitorHandler`:
```go
Monitor: handlers.NewMonitorHandler(monitorRepo, ingestionService, jobQueue, ruleEngine, uploadLogRepo),
```

- [ ] **Step 8: Verificar que compila y los tests existentes siguen pasando**

Run: `cd backend && go build ./... && go test ./...`
Expected: build limpio, todos los tests OK (incluidos los de las Tareas 1-2).

- [ ] **Step 9: Verificación manual**

Con el backend corriendo localmente (ver notas de verificación de tareas anteriores de este roadmap si hace falta levantar Mongo/Redis):
1. Crear o usar un monitor CSV existente sin datos. Subir un CSV con columnas `monto,cliente` → debería ingerirse normal (primer upload, sin base para comparar).
2. Subir OTRO CSV al mismo monitor con columnas distintas (ej. `monto,cliente,extra`) → debería responder `422` con `schemaDiff.extraFields: ["extra"]`, y NO agregar registros nuevos al monitor.
3. Subir un CSV con las columnas correctas pero una fila con `monto` no numérico → debería ingerirse el resto de las filas, y la respuesta debería reflejar `rowsRejected > 0`.

Documentar en el reporte qué se observó en cada paso.

- [ ] **Step 10: Commit**

```bash
git add backend/internal/services/ingestion_service.go backend/internal/handlers/monitor_handler.go backend/cmd/server/main.go
git commit -m "feat(bitacora): validar estructura y filas antes de ingerir, registrar en la bitácora"
```

---

### Task 5: Endpoint de verificación previa (`/upload/check`)

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Consumes: `IngestCSV`/`IngestTXT`/`IngestExcel` con `dryRun=true` (Tarea 4).
- Produces: `POST /monitors/:id/upload/check` — Tarea 9 (frontend) lo consume.

Sin TDD — plomería sobre lógica ya testeada. Verificación manual.

- [ ] **Step 1: Agregar el handler `UploadCheck`**

Agregar a `backend/internal/handlers/monitor_handler.go`, después de `UploadData`:

```go
// UploadCheck corre la misma validación que UploadData (comparación de
// estructura + validación por fila) SIN escribir nada en Mongo ni en la
// bitácora — un "dry run" para que el frontend alerte al usuario antes
// de comprometer la subida real.
func (h *MonitorHandler) UploadCheck(c *fiber.Ctx) error {
	id, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), id)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileHeader, err := c.FormFile("file")
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "file is required"})
	}

	f, err := fileHeader.Open()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "opening file"})
	}
	defer func() { _ = f.Close() }()

	rawBytes, err := io.ReadAll(f)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "reading file"})
	}

	var outcome *services.IngestOutcome
	switch monitor.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, &memFile{Reader: bytes.NewReader(rawBytes)}, true)
	default:
		// JSON no tiene validación por fila en esta iteración (ver spec)
		// ni necesita dry-run — se sube directo.
		return c.JSON(fiber.Map{"match": true, "totalRows": 0, "rowsThatWouldPass": 0, "rowsThatWouldFail": 0})
	}

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return c.JSON(fiber.Map{
		"match":             outcome.SchemaDiff.Match,
		"missingFields":     outcome.SchemaDiff.MissingFields,
		"extraFields":       outcome.SchemaDiff.ExtraFields,
		"typeMismatches":    outcome.SchemaDiff.TypeMismatches,
		"totalRows":         outcome.TotalRows,
		"rowsThatWouldPass": outcome.RowsAccepted,
		"rowsThatWouldFail": len(outcome.RowRejections),
	})
}
```

- [ ] **Step 2: Registrar la ruta**

En `backend/internal/router/router.go`, agregar justo después de la línea `monitors.Post("/:id/upload", middleware.RequireComplianceOrAbove(), h.Monitor.UploadData)`:

```go
	monitors.Post("/:id/upload/check", middleware.RequireComplianceOrAbove(), h.Monitor.UploadCheck)
```

- [ ] **Step 3: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 4: Verificación manual**

Con el backend corriendo: `curl -X POST http://localhost:8080/api/v1/monitors/<id>/upload/check -H "Authorization: Bearer <token>" -F "file=@archivo.csv"` contra un monitor con base ya establecida — confirmar que devuelve `match:false` con el diff correcto para un archivo con columnas distintas, y que el monitor NO se modificó (mismo `record_count` que antes).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go backend/internal/router/router.go
git commit -m "feat(bitacora): endpoint de verificación previa (dry-run)"
```

---

### Task 6: Endpoints de listado y aprobación de la bitácora

**Files:**
- Modify: `backend/internal/handlers/monitor_handler.go`
- Modify: `backend/internal/router/router.go`

**Interfaces:**
- Consumes: `UploadLogRepository` (Tarea 3), `IngestCSV`/`IngestTXT`/`IngestExcel` (Tarea 4).
- Produces: `GET /upload-log`, `POST /monitors/:id/upload-log/:logId/approve` — Tarea 8 (frontend) los consume.

Sin TDD — plomería. Verificación manual.

- [ ] **Step 1: Agregar `UploadLogList`**

Agregar a `backend/internal/handlers/monitor_handler.go`:

```go
// UploadLogList devuelve la bitácora completa, opcionalmente filtrada
// por monitor vía ?monitorId=. Mismo criterio de acceso que
// IngestionHistory (cualquier usuario autenticado puede ver — no hay
// restricción de rol para lectura, solo para aprobar).
func (h *MonitorHandler) UploadLogList(c *fiber.Ctx) error {
	var monitorID *primitive.ObjectID
	if q := c.Query("monitorId"); q != "" {
		id, err := primitive.ObjectIDFromHex(q)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitorId"})
		}
		monitorID = &id
	}

	entries, err := h.uploadLogRepo.List(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(entries)
}
```

- [ ] **Step 2: Agregar `UploadLogApprove`**

Agregar a `backend/internal/handlers/monitor_handler.go`:

```go
// UploadLogApprove re-ingiere el archivo guardado de una entrada
// rejected_structure, tratando su estructura como la nueva base del
// monitor. Se limpia monitor.Schema antes de re-ingerir: compareSchema
// trata un schema vacío como "primer upload" (siempre match), así el
// mismo camino de Ingest* que ya existe detecta y fija la nueva
// estructura sin necesitar un parámetro "forzar" aparte.
func (h *MonitorHandler) UploadLogApprove(c *fiber.Ctx) error {
	monitorID, err := primitive.ObjectIDFromHex(c.Params("id"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid monitor ID"})
	}
	logID, err := primitive.ObjectIDFromHex(c.Params("logId"))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid log ID"})
	}

	entry, err := h.uploadLogRepo.Get(c.Context(), logID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "upload log entry not found"})
	}
	if entry.Status != models.UploadStatusRejectedStructure {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "solo se pueden aprobar entradas rechazadas por estructura"})
	}

	monitor, err := h.monitorRepo.FindByID(c.Context(), monitorID)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "monitor not found"})
	}

	fileBytes, err := h.uploadLogRepo.DownloadRejectedFile(c.Context(), entry)
	if err != nil {
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	}

	monitor.Schema = nil // fuerza a compareSchema a tratar esto como primer upload

	var outcome *services.IngestOutcome
	switch entry.SourceType {
	case models.SourceCSV:
		outcome, err = h.ingestionService.IngestCSV(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	case models.SourceExcel:
		outcome, err = h.ingestionService.IngestExcel(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	case models.SourceTXT:
		outcome, err = h.ingestionService.IngestTXT(c.Context(), monitor, &memFile{Reader: bytes.NewReader(fileBytes)}, false)
	default:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "tipo de fuente no soportado para aprobación"})
	}
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if approveErr := h.uploadLogRepo.MarkApproved(c.Context(), logID, outcome.TotalRows, outcome.RowsAccepted, len(outcome.RowRejections), outcome.RowRejections); approveErr != nil {
		log.Printf("bitacora: marcando entrada aprobada: %v", approveErr)
	}

	return c.JSON(fiber.Map{
		"recordsIngested": outcome.RowsAccepted,
		"rowsRejected":    len(outcome.RowRejections),
		"schema":          monitor.Schema,
	})
}
```

- [ ] **Step 3: Registrar las rutas**

En `backend/internal/router/router.go`, agregar debajo de `monitors.Get("/:id/data", h.Monitor.GetData)`:

```go
	monitors.Post("/:id/upload-log/:logId/approve", middleware.RequireComplianceOrAbove(), h.Monitor.UploadLogApprove)
```

Y reemplazar el comentario/línea existente:
```go
	// Ingestion history
	protected.Get("/ingestion-history", h.Monitor.IngestionHistory)
```
por:
```go
	// Ingestion history
	protected.Get("/ingestion-history", h.Monitor.IngestionHistory)
	protected.Get("/upload-log", h.Monitor.UploadLogList)
```

- [ ] **Step 4: Verificar que compila**

Run: `cd backend && go build ./...`
Expected: sin errores.

- [ ] **Step 5: Verificación manual**

1. Repetir el Step 9.2 de la Tarea 4 (subir un archivo con estructura distinta, confirmar rechazo).
2. `curl http://localhost:8080/api/v1/upload-log -H "Authorization: Bearer <token>"` → confirmar que aparece la entrada con `status: "rejected_structure"`.
3. `curl -X POST http://localhost:8080/api/v1/monitors/<id>/upload-log/<logId>/approve -H "Authorization: Bearer <compliance-token>"` → confirmar `200`, que el monitor ahora tiene la nueva estructura (`GET /monitors/<id>`), y que la entrada de la bitácora pasó a `status: "approved"`.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/handlers/monitor_handler.go backend/internal/router/router.go
git commit -m "feat(bitacora): endpoints de listado y aprobación"
```

---

### Task 7: Cliente API del frontend

**Files:**
- Modify: `frontend/src/lib/api/monitors.ts`
- Create: `frontend/src/lib/types.ts:` (modificar — agregar tipos, ver Step 1)

**Interfaces:**
- Produces: `monitorsApi.uploadCheck`, `monitorsApi.approveUploadLog`, tipos `UploadLogEntry`/`SchemaDiff`/`RowRejection`/`UploadCheckResult` — Tareas 8-9 los consumen.

Sin tests — mismo criterio que el resto de `lib/api/*.ts` (sin convención de testing acá).

- [ ] **Step 1: Agregar los tipos**

Buscar la definición de `SchemaField` en `frontend/src/lib/types.ts` y agregar justo después:

```ts
export type UploadStatus = "accepted" | "partial" | "rejected_structure" | "approved";

export interface FieldTypeMismatch {
  field: string;
  expectedType: string;
  actualType: string;
}

export interface SchemaDiff {
  match: boolean;
  missingFields?: string[];
  extraFields?: string[];
  typeMismatches?: FieldTypeMismatch[];
}

export interface RowRejection {
  rowIndex: number;
  field: string;
  reason: string;
}

export interface UploadLogEntry {
  id: string;
  monitorId: string;
  monitorName: string;
  fileName: string;
  sourceType: SourceType;
  uploadedByEmail: string;
  uploadedAt: string;
  status: UploadStatus;
  totalRows: number;
  rowsAccepted: number;
  rowsRejected: number;
  schemaDiff?: SchemaDiff;
  rowRejections?: RowRejection[];
}

export interface UploadCheckResult {
  match: boolean;
  missingFields?: string[];
  extraFields?: string[];
  typeMismatches?: FieldTypeMismatch[];
  totalRows: number;
  rowsThatWouldPass: number;
  rowsThatWouldFail: number;
}
```

- [ ] **Step 2: Agregar métodos al cliente**

`ApiClient.request` (`frontend/src/lib/api/client.ts:29`) es `private` — no se puede llamar desde `monitors.ts`. `ApiClient.upload<T>(path, file)` (`client.ts:118`) ya es genérico en `T` y en `path` (arma el mismo `FormData` con la key `"file"` que necesitamos), así que se reusa tal cual con el path nuevo, sin tocar `client.ts`.

En `frontend/src/lib/api/monitors.ts`, agregar el import de los tipos nuevos al bloque `import type {...} from "@/lib/types";` (`UploadCheckResult` — no hace falta importar `UploadLogEntry`/`RowRejection`/`SchemaDiff` acá, van en `upload-log.ts`, Tarea 8) y agregar, después de `upload:`:

```ts
  uploadCheck: (id: string, file: File) =>
    api.upload<UploadCheckResult>(`/monitors/${id}/upload/check`, file),
```

Crear también, en un archivo nuevo `frontend/src/lib/api/upload-log.ts`:

```ts
import { api } from "./client";
import type { UploadLogEntry } from "@/lib/types";

export const uploadLogApi = {
  list: () => api.get<UploadLogEntry[]>("/upload-log"),

  approve: (monitorId: string, logId: string) =>
    api.post<{ recordsIngested: number; rowsRejected: number }>(
      `/monitors/${monitorId}/upload-log/${logId}/approve`,
      {},
    ),
};
```

- [ ] **Step 3: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 4: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/api/monitors.ts frontend/src/lib/api/upload-log.ts
git commit -m "feat(bitacora): cliente API del frontend"
```

---

### Task 8: Extender `/uploads` con filtro por monitor, estado, y tabla detallada

**Files:**
- Modify: `frontend/src/app/(dashboard)/uploads/page.tsx`

**Interfaces:**
- Consumes: `uploadLogApi.list`, `uploadLogApi.approve` (Tarea 7), `UploadLogEntry`/`SchemaDiff`/`RowRejection` (Tarea 7), `useAuthStore` (existente).

Sin tests (componente React, sin convención de testing de componentes en este proyecto).

- [ ] **Step 1: Agregar imports y estado nuevo**

En `frontend/src/app/(dashboard)/uploads/page.tsx`, cambiar la línea `import { useEffect, useMemo, useState } from "react";` por:

```tsx
import { Fragment, useEffect, useMemo, useState } from "react";
```

(la tabla detallada del Step 6 devuelve 2 filas `<tr>` por entrada — activa + detalle expandible — así que cada iteración de `.map()` necesita un `<Fragment key={...}>` explícito envolviéndolas, no el shorthand `<>` que no acepta `key`.)

Agregar al bloque de imports:

```tsx
import { useAuthStore } from "@/stores/auth-store";
import { uploadLogApi } from "@/lib/api/upload-log";
import { STATUS_CLASSES } from "@/lib/semantic-colors";
import { ChevronDown, ChevronRight, CheckCircle2, XCircle, AlertTriangle } from "lucide-react";
import type { UploadLogEntry, UploadStatus } from "@/lib/types";
```

Después de la declaración de `const [sortDir, setSortDir] = useState<SortDir>("desc");`, agregar:

```tsx
  // Bitácora detallada
  const [logEntries, setLogEntries] = useState<UploadLogEntry[]>([]);
  const [monitorFilter, setMonitorFilter] = useState<string>("all");
  const [statusFilter, setStatusFilter] = useState<string>("all");
  const [expandedId, setExpandedId] = useState<string | null>(null);
  const [approvingId, setApprovingId] = useState<string | null>(null);
  const user = useAuthStore((s) => s.user);
  const canApprove = user != null && user.role !== "viewer";
```

- [ ] **Step 2: Cargar la bitácora en el mismo `useEffect`**

Reemplazar el `useEffect` existente (`useEffect(() => { async function load() {...`) por:

```tsx
  useEffect(() => {
    async function load() {
      try {
        const [ingestion, log] = await Promise.all([
          api.get<IngestionEntry[]>("/ingestion-history"),
          uploadLogApi.list(),
        ]);
        setEntries(ingestion);
        setLogEntries(log);
      } catch {
        setError("No se pudo cargar el historial de cargas");
      } finally {
        setLoading(false);
      }
    }
    load();
  }, []);
```

- [ ] **Step 3: Derivar la lista de monitores y filtrar la bitácora**

Agregar después del `useMemo` de `sourceTypes`:

```tsx
  // Monitores únicos, derivados de la bitácora, para el filtro
  const logMonitors = useMemo(() => {
    const map = new Map<string, string>();
    for (const e of logEntries) map.set(e.monitorId, e.monitorName);
    return Array.from(map.entries()).sort((a, b) => a[1].localeCompare(b[1]));
  }, [logEntries]);

  const filteredLog = useMemo(() => {
    let result = logEntries;
    if (monitorFilter !== "all") {
      result = result.filter((e) => e.monitorId === monitorFilter);
    }
    if (statusFilter !== "all") {
      result = result.filter((e) => e.status === statusFilter);
    }
    if (search) {
      const q = search.toLowerCase();
      result = result.filter(
        (e) =>
          e.monitorName.toLowerCase().includes(q) ||
          e.fileName.toLowerCase().includes(q),
      );
    }
    return [...result].sort(
      (a, b) => new Date(b.uploadedAt).getTime() - new Date(a.uploadedAt).getTime(),
    );
  }, [logEntries, monitorFilter, statusFilter, search]);
```

- [ ] **Step 4: Agregar los helpers de estado y la función de aprobar**

Agregar cerca de `formatNumber`:

```tsx
const STATUS_LABELS: Record<UploadStatus, string> = {
  accepted: "Aceptado",
  partial: "Parcial",
  rejected_structure: "Rechazado",
  approved: "Aprobado",
};

const STATUS_ICONS: Record<UploadStatus, typeof CheckCircle2> = {
  accepted: CheckCircle2,
  partial: AlertTriangle,
  rejected_structure: XCircle,
  approved: CheckCircle2,
};

// success/warning/danger/info: eje de estado, no de riesgo — ver CLAUDE.md.
const STATUS_KIND: Record<UploadStatus, "success" | "warning" | "danger" | "info"> = {
  accepted: "success",
  partial: "warning",
  rejected_structure: "danger",
  approved: "info",
};

function formatDateTime(iso: string) {
  return new Date(iso).toLocaleString("es-PA", {
    day: "2-digit",
    month: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}
```

Dentro del componente, después de `toggleSort`, agregar:

```tsx
  async function handleApprove(entry: UploadLogEntry) {
    setApprovingId(entry.id);
    try {
      await uploadLogApi.approve(entry.monitorId, entry.id);
      const log = await uploadLogApi.list();
      setLogEntries(log);
    } catch {
      setError("No se pudo aprobar la entrada");
    } finally {
      setApprovingId(null);
    }
  }
```

- [ ] **Step 5: Agregar los filtros de monitor y estado a la barra de filtros existente**

Buscar el bloque de filtros (`{/* Filters */}`) y agregar, después del `<Select value={sourceFilter} ...>` existente:

```tsx
          <Select value={monitorFilter} onValueChange={setMonitorFilter}>
            <SelectTrigger className="w-[180px]">
              <SelectValue placeholder="Monitor" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los monitores</SelectItem>
              {logMonitors.map(([id, name]) => (
                <SelectItem key={id} value={id}>
                  {name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Select value={statusFilter} onValueChange={setStatusFilter}>
            <SelectTrigger className="w-[160px]">
              <SelectValue placeholder="Estado" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">Todos los estados</SelectItem>
              <SelectItem value="accepted">Aceptado</SelectItem>
              <SelectItem value="partial">Parcial</SelectItem>
              <SelectItem value="rejected_structure">Rechazado</SelectItem>
              <SelectItem value="approved">Aprobado</SelectItem>
            </SelectContent>
          </Select>
```

- [ ] **Step 6: Agregar la tabla detallada, debajo de la tabla agregada existente**

Justo antes del cierre `</div>` final del componente (después del bloque `{/* Grid table */}` existente, que queda intacto), agregar:

```tsx
        {/* Bitácora detallada por archivo */}
        <div className="mt-8">
          <h2 className="mb-3 text-sm font-semibold text-muted-foreground">
            Bitácora de archivos subidos
          </h2>
          {filteredLog.length === 0 ? (
            <Card>
              <CardContent className="flex flex-col items-center justify-center py-12">
                <Upload className="mb-3 h-10 w-10 text-muted-foreground/40" />
                <p className="text-sm text-muted-foreground">
                  No hay archivos en la bitácora con los filtros aplicados
                </p>
              </CardContent>
            </Card>
          ) : (
            <Card>
              <div className="overflow-x-auto">
                <table className="w-full">
                  <thead>
                    <tr className="border-b bg-muted/50">
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Archivo
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Monitor
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Fecha/hora
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Subido por
                      </th>
                      <th className="px-4 py-3 text-left text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Estado
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Totales
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Aceptados
                      </th>
                      <th className="px-4 py-3 text-right text-xs font-medium uppercase tracking-wider text-muted-foreground">
                        Rechazados
                      </th>
                      <th className="px-4 py-3" />
                    </tr>
                  </thead>
                  <tbody>
                    {filteredLog.map((entry) => {
                      const StatusIcon = STATUS_ICONS[entry.status];
                      const expanded = expandedId === entry.id;
                      const hasDetail =
                        (entry.schemaDiff && !entry.schemaDiff.match) ||
                        (entry.rowRejections && entry.rowRejections.length > 0);
                      return (
                        <Fragment key={entry.id}>
                          <tr className="border-b transition-colors hover:bg-muted/30 last:border-0">
                            <td className="px-4 py-3">
                              <div className="flex items-center gap-2">
                                {hasDetail && (
                                  <button
                                    onClick={() => setExpandedId(expanded ? null : entry.id)}
                                    className="text-muted-foreground hover:text-foreground"
                                  >
                                    {expanded ? (
                                      <ChevronDown className="h-4 w-4" />
                                    ) : (
                                      <ChevronRight className="h-4 w-4" />
                                    )}
                                  </button>
                                )}
                                <span className="text-sm font-medium">{entry.fileName}</span>
                              </div>
                            </td>
                            <td className="px-4 py-3">
                              <a
                                href={`/monitors/${entry.monitorId}`}
                                className="text-sm text-primary hover:underline"
                              >
                                {entry.monitorName}
                              </a>
                            </td>
                            <td className="px-4 py-3 text-sm text-muted-foreground">
                              {formatDateTime(entry.uploadedAt)}
                            </td>
                            <td className="px-4 py-3 text-sm text-muted-foreground">
                              {entry.uploadedByEmail}
                            </td>
                            <td className="px-4 py-3">
                              <Badge
                                variant="outline"
                                className={cn("gap-1", STATUS_CLASSES[STATUS_KIND[entry.status]])}
                              >
                                <StatusIcon className="h-3 w-3" />
                                {STATUS_LABELS[entry.status]}
                              </Badge>
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums">
                              {formatNumber(entry.totalRows)}
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums text-success-fg">
                              {formatNumber(entry.rowsAccepted)}
                            </td>
                            <td className="px-4 py-3 text-right text-sm tabular-nums text-danger-fg">
                              {formatNumber(entry.rowsRejected)}
                            </td>
                            <td className="px-4 py-3 text-right">
                              {entry.status === "rejected_structure" && canApprove && (
                                <button
                                  onClick={() => handleApprove(entry)}
                                  disabled={approvingId === entry.id}
                                  className="rounded-md border px-2 py-1 text-xs font-medium hover:bg-accent disabled:opacity-50"
                                >
                                  {approvingId === entry.id ? "Aprobando…" : "Aprobar"}
                                </button>
                              )}
                            </td>
                          </tr>
                          {expanded && hasDetail && (
                            <tr className="border-b bg-muted/20 last:border-0">
                              <td colSpan={9} className="px-4 py-3">
                                {entry.schemaDiff && !entry.schemaDiff.match && (
                                  <div className="mb-2 space-y-1 text-xs">
                                    {entry.schemaDiff.missingFields && entry.schemaDiff.missingFields.length > 0 && (
                                      <p>
                                        <span className="font-medium">Faltan columnas: </span>
                                        {entry.schemaDiff.missingFields.join(", ")}
                                      </p>
                                    )}
                                    {entry.schemaDiff.extraFields && entry.schemaDiff.extraFields.length > 0 && (
                                      <p>
                                        <span className="font-medium">Columnas de más: </span>
                                        {entry.schemaDiff.extraFields.join(", ")}
                                      </p>
                                    )}
                                    {entry.schemaDiff.typeMismatches && entry.schemaDiff.typeMismatches.length > 0 && (
                                      <p>
                                        <span className="font-medium">Tipo distinto: </span>
                                        {entry.schemaDiff.typeMismatches
                                          .map((m) => `${m.field} (esperado ${m.expectedType}, encontrado ${m.actualType})`)
                                          .join(", ")}
                                      </p>
                                    )}
                                  </div>
                                )}
                                {entry.rowRejections && entry.rowRejections.length > 0 && (
                                  <ul className="space-y-0.5 text-xs text-muted-foreground">
                                    {entry.rowRejections.map((r, i) => (
                                      <li key={i}>
                                        Fila {r.rowIndex}: {r.field} — {r.reason}
                                      </li>
                                    ))}
                                  </ul>
                                )}
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </Card>
          )}
        </div>
```

- [ ] **Step 7: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 8: Verificar lint**

Run: `cd frontend && npm run lint`
Expected: 0 errores nuevos (warnings preexistentes sin cambios).

- [ ] **Step 9: Verificación manual**

Levantar el dev server, navegar a `/uploads`, confirmar: el filtro de monitor lista los monitores con entradas en la bitácora, el filtro de estado filtra correctamente, una entrada `rejected_structure` es expandible y muestra el diff, el botón "Aprobar" aparece solo si el usuario no es viewer.

- [ ] **Step 10: Commit**

```bash
git add frontend/src/app/'(dashboard)'/uploads/page.tsx
git commit -m "feat(bitacora): extender /uploads con filtros, tabla detallada y aprobación"
```

---

### Task 9: Verificación previa automática al soltar el archivo

**Files:**
- Modify: `frontend/src/components/tabs/content/monitor-tab-content.tsx`

**Interfaces:**
- Consumes: `monitorsApi.uploadCheck` (Tarea 7), `UploadCheckResult` (Tarea 7), `AlertDialog` (shadcn, ya existente en el proyecto).

Sin tests (componente React).

- [ ] **Step 1: Agregar imports**

En `frontend/src/components/tabs/content/monitor-tab-content.tsx`, agregar al bloque de imports:

```tsx
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog";
import type { UploadCheckResult } from "@/lib/types";
```

- [ ] **Step 2: Agregar estado del diálogo de verificación**

Cerca de la declaración de `uploadResult`, agregar:

```tsx
  const [pendingFile, setPendingFile] = useState<File | null>(null);
  const [checkResult, setCheckResult] = useState<UploadCheckResult | null>(null);
  const [checking, setChecking] = useState(false);
```

- [ ] **Step 3: Reescribir `onDrop` para verificar antes de subir**

Reemplazar el `onDrop` existente (`const onDrop = useCallback(...)`) por:

```tsx
  const onDrop = useCallback(
    async (acceptedFiles: File[]) => {
      if (acceptedFiles.length === 0) return;
      const file = acceptedFiles[0];
      setChecking(true);
      try {
        const check = await monitorsApi.uploadCheck(id, file);
        if (!check.match || check.rowsThatWouldFail > 0) {
          setPendingFile(file);
          setCheckResult(check);
          return;
        }
        await doUpload(file);
      } catch {
        // Si la verificación misma falla (ej. red), se intenta subir
        // directo — el endpoint real vuelve a validar de todas formas.
        await doUpload(file);
      } finally {
        setChecking(false);
      }
    },
    [id],
  );

  async function doUpload(file: File) {
    setUploading(true);
    setUploadResult(null);
    try {
      const result = await monitorsApi.upload(id, file);
      setUploadResult(result);
      loadMonitor();
      loadData();
    } catch {
      toastError("Error al subir archivo");
    } finally {
      setUploading(false);
    }
  }

  async function confirmUploadAnyway() {
    if (!pendingFile) return;
    const file = pendingFile;
    setPendingFile(null);
    setCheckResult(null);
    await doUpload(file);
  }
```

- [ ] **Step 4: Agregar el diálogo de alerta**

Buscar el cierre del `return (` principal del componente (el `</div>` o fragmento final antes del cierre de la función `MonitorTabContent`) y agregar, justo antes de ese cierre final:

```tsx
      <AlertDialog open={pendingFile !== null} onOpenChange={(open) => !open && setPendingFile(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>El archivo no coincide con la estructura esperada</AlertDialogTitle>
            <AlertDialogDescription asChild>
              <div className="space-y-2 text-sm">
                {checkResult?.missingFields && checkResult.missingFields.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Faltan columnas: </span>
                    {checkResult.missingFields.join(", ")}
                  </p>
                )}
                {checkResult?.extraFields && checkResult.extraFields.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Columnas de más: </span>
                    {checkResult.extraFields.join(", ")}
                  </p>
                )}
                {checkResult?.typeMismatches && checkResult.typeMismatches.length > 0 && (
                  <p>
                    <span className="font-medium text-foreground">Tipo distinto: </span>
                    {checkResult.typeMismatches
                      .map((m) => `${m.field} (esperado ${m.expectedType}, encontrado ${m.actualType})`)
                      .join(", ")}
                  </p>
                )}
                {checkResult && checkResult.rowsThatWouldFail > 0 && (
                  <p>
                    {checkResult.rowsThatWouldFail} de {checkResult.totalRows} filas fallarían por tipo
                    de dato incorrecto.
                  </p>
                )}
              </div>
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel onClick={() => setPendingFile(null)}>Cancelar</AlertDialogCancel>
            <AlertDialogAction onClick={confirmUploadAnyway}>Subir de todas formas</AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
```

- [ ] **Step 5: Mostrar un indicador mientras se verifica**

Buscar, dentro del dropzone:

```tsx
                      {uploading ? (
                        <p className="text-sm text-muted-foreground">
                          Procesando archivo...
                        </p>
                      ) : isDragActive ? (
```

Reemplazar por:

```tsx
                      {uploading || checking ? (
                        <p className="text-sm text-muted-foreground">
                          {checking ? "Verificando archivo..." : "Procesando archivo..."}
                        </p>
                      ) : isDragActive ? (
```

- [ ] **Step 6: Verificar que compila**

Run: `cd frontend && npx tsc --noEmit`
Expected: sin errores.

- [ ] **Step 7: Verificación manual**

En el navegador: en un monitor con base ya establecida, soltar un archivo con estructura distinta → debería aparecer el diálogo con el diff, ANTES de que se suba nada (confirmar que no aparece ninguna entrada nueva en `/uploads` hasta hacer click en "Subir de todas formas" o cancelar sin que se suba). Soltar un archivo con estructura correcta → debería subirse directo, sin diálogo.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/components/tabs/content/monitor-tab-content.tsx
git commit -m "feat(bitacora): verificación previa automática antes de subir"
```

---

## Criterio de cierre del lote

- Gate (`go build ./...`, `go test ./...`, `npx tsc --noEmit`, `npm run lint`, `npm run build`) sin bloqueantes en las 9 tareas.
- Recorrido manual (además de las verificaciones puntuales de cada tarea):
  - Subir a un monitor nuevo (sin base) cualquier archivo → se ingiere sin fricción, queda la base establecida.
  - Subir un archivo con columnas distintas al mismo monitor → diálogo de alerta ANTES de subir; cancelar no deja rastro; "subir de todas formas" deja una entrada `rejected_structure` en la bitácora, sin afectar los datos del monitor.
  - Aprobar esa entrada (usuario compliance/admin) → se re-ingiere, el monitor adopta la nueva estructura, la entrada pasa a `approved`.
  - Subir un archivo con estructura correcta pero una fila con un valor de tipo incorrecto → se ingieren las filas válidas, la entrada queda `partial` con el detalle de la fila rechazada.
  - Filtrar la bitácora por monitor y por estado en `/uploads`, confirmar que ambos filtros funcionan y se pueden combinar.
  - Confirmar que un usuario `viewer` ve la bitácora pero no ve el botón "Aprobar".

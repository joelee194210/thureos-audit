package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/thureos/compliance/internal/models"
	"github.com/thureos/compliance/internal/repository"
	"github.com/xuri/excelize/v2"
	"go.mongodb.org/mongo-driver/bson"
)

type IngestionService struct {
	monitorRepo *repository.MonitorRepository
	// httpClient backs DetectAPISchema. Defaults to the same SSRF-safe
	// client monitor_puller.go uses for real pulls; tests in this package
	// may swap it for a plain client to talk to an httptest server, which
	// listens on loopback and would otherwise be refused by the SSRF guard.
	httpClient *http.Client
}

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

func NewIngestionService(monitorRepo *repository.MonitorRepository) *IngestionService {
	return &IngestionService{monitorRepo: monitorRepo, httpClient: newSafeHTTPClient()}
}

// ErrPushModeSchemaDetection is returned by DetectAPISchema when asked to
// probe a push-mode config — distinct from network/response errors so the
// handler can map it to 400 instead of a gateway error status.
var ErrPushModeSchemaDetection = errors.New("la detección automática solo está disponible para modo pull; en modo push, el schema se detecta con el primer envío")

// ErrMalformedFile marks an error as a file parsing/reading failure (as
// opposed to a downstream Mongo error) so handlers can map it to 400
// instead of the generic 500.
var ErrMalformedFile = errors.New("el archivo no se pudo leer o parsear")

// maxSchemaDetectSampleRows caps how many records DetectAPISchema feeds into
// detectSchemaFromJSON — a preview only needs enough rows to see the shape
// of the data, not the whole response.
const maxSchemaDetectSampleRows = 50

// DetectAPISchema makes one live request to cfg's configured pull endpoint
// and infers a schema from the response, without persisting anything —
// callers use it to let a user confirm a schema before a monitor is created.
func (s *IngestionService) DetectAPISchema(ctx context.Context, cfg models.SourceConfig) ([]models.SchemaField, int, error) {
	if cfg.Mode != models.APIModePull {
		return nil, 0, ErrPushModeSchemaDetection
	}

	method := cfg.PullMethod
	if method == "" {
		method = http.MethodGet
	}

	req, err := http.NewRequestWithContext(ctx, method, cfg.PullURL, nil)
	if err != nil {
		return nil, 0, fmt.Errorf("armando la petición de prueba: %w", err)
	}

	switch cfg.PullAuthType {
	case models.APIAuthAPIKey:
		req.Header.Set(cfg.PullAuthHeaderName, cfg.PullAuthValue)
	case models.APIAuthBearer:
		req.Header.Set("Authorization", "Bearer "+cfg.PullAuthValue)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("no se pudo contactar %s: %w", cfg.PullURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, 0, fmt.Errorf("el API externo respondió HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Read one byte past the cap: if it's present, the body was truncated
	// and any JSON error below is an artifact of that, not of the API's
	// actual response — the two need different messages.
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxPullResponseBytes+1))
	if err != nil {
		return nil, 0, fmt.Errorf("leyendo la respuesta del API: %w", err)
	}
	if len(raw) > maxPullResponseBytes {
		return nil, 0, fmt.Errorf("la respuesta del API supera el límite de %d MB permitido para la detección de schema; probá acotarla con rootPath o un endpoint paginado", maxPullResponseBytes/(1024*1024))
	}

	// debt: interface{} es deliberado — la detección de schema debe aceptar
	// JSON de forma arbitraria; no existe un struct concreto para payloads
	// de terceros. Revisar -> si los monitores llegan a tener contratos de
	// ingesta tipados.
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, 0, fmt.Errorf("la respuesta del API no es JSON válido: %w", err)
	}

	extracted, err := extractRootPath(parsed, cfg.RootPath)
	if err != nil {
		return nil, 0, fmt.Errorf("extrayendo rootPath: %w", err)
	}

	if extracted == nil {
		return nil, 0, fmt.Errorf("la respuesta no tiene registros para inferir el schema")
	}

	records, err := toRecordSlice(extracted)
	if err != nil {
		return nil, 0, fmt.Errorf("interpretando los registros del API: %w", err)
	}

	if len(records) == 0 {
		return nil, 0, fmt.Errorf("la respuesta no tiene registros para inferir el schema")
	}

	if len(records) > maxSchemaDetectSampleRows {
		records = records[:maxSchemaDetectSampleRows]
	}

	return detectSchemaFromJSON(records), len(records), nil
}

// IngestCSV parses a CSV file, detects schema, and stores data
func (s *IngestionService) IngestCSV(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	return s.ingestDelimited(ctx, monitor, file, ',', dryRun)
}

// IngestTXT parses a delimited text file (default: tab-separated), detects
// schema, and stores data. Same parser as IngestCSV — only the default
// delimiter differs, and SourceConfig.Delimiter always overrides either default.
func (s *IngestionService) IngestTXT(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	return s.ingestDelimited(ctx, monitor, file, '\t', dryRun)
}

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
			return nil, fmt.Errorf("reading headers: %w: %w", ErrMalformedFile, err)
		}
		headers = h
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading row: %w: %w", ErrMalformedFile, err)
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
	var count int
	if len(documents) > 0 {
		var err error
		count, err = s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
		if err != nil {
			return nil, fmt.Errorf("inserting data: %w", err)
		}
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

// IngestJSON parses a JSON document, optionally drilling into a nested
// array via SourceConfig.RootPath, detects schema, and stores data.
// Takes io.Reader (not multipart.File) so it can be reused for HTTP
// request/response bodies from API push and pull ingestion, not just
// uploaded files — multipart.File already satisfies io.Reader, so existing
// callers (UploadData) need no changes.
func (s *IngestionService) IngestJSON(ctx context.Context, monitor *models.Monitor, file io.Reader) (int, error) {
	raw, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("reading JSON: %w", err)
	}

	// debt: interface{} es deliberado — aquí se ingesta JSON arbitrario del
	// usuario antes de conocer el schema. Revisar -> solo si la ingesta pasa a
	// payloads tipados.
	var parsed interface{}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return 0, fmt.Errorf("parsing JSON: %w", err)
	}

	rootPath := ""
	if monitor.SourceConfig != nil {
		rootPath = monitor.SourceConfig.RootPath
	}

	extracted, err := extractRootPath(parsed, rootPath)
	if err != nil {
		return 0, fmt.Errorf("extracting rootPath: %w", err)
	}

	// A bare top-level `null` (or a rootPath resolving to null) is treated
	// as zero records rather than an error — matches the pre-rootPath
	// behavior of unmarshaling into a nil slice, and "no data yet" is a
	// legitimate response from a real API, not a malformed one.
	if extracted == nil {
		return 0, nil
	}

	records, err := toRecordSlice(extracted)
	if err != nil {
		return 0, fmt.Errorf("interpreting JSON records: %w", err)
	}

	if len(records) == 0 {
		return 0, nil
	}

	if len(monitor.Schema) == 0 {
		monitor.Schema = detectSchemaFromJSON(records)
	}

	documents := make([]interface{}, 0, len(records))
	for _, record := range records {
		record["_ingested_at"] = time.Now()
		documents = append(documents, record)
	}

	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return 0, fmt.Errorf("inserting data: %w", err)
	}

	now := time.Now()
	if err := s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	}); err != nil {
		log.Printf("ingestion: actualizando estado del monitor: %v", err)
	}

	return count, nil
}

// extractRootPath navigates a parsed JSON value by dot-separated path
// (e.g. "data.records") and returns the value found there. An empty path
// returns v unchanged — this is the default, matching today's behavior of
// treating the JSON body itself as the records.
func extractRootPath(v interface{}, path string) (interface{}, error) {
	if path == "" {
		return v, nil
	}
	current := v
	for _, key := range strings.Split(path, ".") {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("rootPath %q: expected an object before key %q, got %T", path, key, current)
		}
		val, exists := m[key]
		if !exists {
			return nil, fmt.Errorf("rootPath %q: key %q not found", path, key)
		}
		current = val
	}
	return current, nil
}

// toRecordSlice coerces a parsed JSON value into a slice of records. Matches
// the pre-existing IngestJSON fallback: an array of objects ingests as-is,
// a single bare object ingests as one record.
func toRecordSlice(v interface{}) ([]map[string]interface{}, error) {
	switch val := v.(type) {
	case []interface{}:
		records := make([]map[string]interface{}, 0, len(val))
		for _, item := range val {
			m, ok := item.(map[string]interface{})
			if !ok {
				return nil, fmt.Errorf("expected an array of objects, got an element of type %T", item)
			}
			records = append(records, m)
		}
		return records, nil
	case map[string]interface{}:
		return []map[string]interface{}{val}, nil
	default:
		return nil, fmt.Errorf("expected a JSON array or object at the root (or at rootPath), got %T", v)
	}
}

// IngestExcel parses an XLSX file and stores data
func (s *IngestionService) IngestExcel(ctx context.Context, monitor *models.Monitor, file multipart.File, dryRun bool) (*IngestOutcome, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return nil, fmt.Errorf("opening Excel: %w: %w", ErrMalformedFile, err)
	}
	defer func() { _ = f.Close() }()

	sheetName := f.GetSheetName(0)
	if monitor.SourceConfig != nil && monitor.SourceConfig.SheetName != "" {
		sheetName = monitor.SourceConfig.SheetName
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return nil, fmt.Errorf("reading Excel rows: %w: %w", ErrMalformedFile, err)
	}

	if len(rows) < 2 {
		return nil, fmt.Errorf("%w: excel file has no data rows", ErrMalformedFile)
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
	var count int
	if len(documents) > 0 {
		var err error
		count, err = s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
		if err != nil {
			return nil, fmt.Errorf("inserting data: %w", err)
		}
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

// Schema detection helpers

// detectSchemaFromCSV expects fieldNames already sanitized and deduped
// (see dedupeFieldNames) — it does not re-derive names from raw headers.
func detectSchemaFromCSV(fieldNames []string, rows [][]string) []models.SchemaField {
	schema := make([]models.SchemaField, len(fieldNames))
	for i, name := range fieldNames {
		fieldType := models.FieldString
		sample := ""

		for _, row := range rows {
			if i < len(row) && row[i] != "" {
				sample = row[i]
				fieldType = inferType(row[i])
				break
			}
		}

		schema[i] = models.SchemaField{
			Name:     name,
			Type:     fieldType,
			Required: true,
			Sample:   sample,
		}
	}
	return schema
}

// dedupeFieldNames sanitizes each header and disambiguates collisions
// (blank or repeated header cells are common in real exports) by suffixing
// repeats with _2, _3, ... — otherwise two columns writing to the same BSON
// key would silently overwrite each other when the document is built.
func dedupeFieldNames(headers []string) []string {
	seen := make(map[string]int, len(headers))
	names := make([]string, len(headers))
	for i, h := range headers {
		name := sanitizeFieldName(h)
		if name == "" {
			name = fmt.Sprintf("col_%d", i+1)
		}
		seen[name]++
		if n := seen[name]; n > 1 {
			name = fmt.Sprintf("%s_%d", name, n)
		}
		names[i] = name
	}
	return names
}

func detectSchemaFromJSON(records []map[string]interface{}) []models.SchemaField {
	fieldMap := make(map[string]models.FieldType)
	sampleMap := make(map[string]string)

	for _, record := range records {
		for key, val := range record {
			if _, exists := fieldMap[key]; !exists {
				switch v := val.(type) {
				case float64:
					fieldMap[key] = models.FieldNumber
				case bool:
					fieldMap[key] = models.FieldBoolean
				case string:
					// JSON already distinguishes numbers/booleans by type, so a
					// string value only needs the date check inferType also
					// runs for CSV — a bare "string" default would otherwise
					// misclassify every ISO date/timestamp field from an API.
					if looksLikeDate(v) {
						fieldMap[key] = models.FieldDate
					} else {
						fieldMap[key] = models.FieldString
					}
				default:
					fieldMap[key] = models.FieldString
				}
				sampleMap[key] = fmt.Sprintf("%v", val)
			}
		}
	}

	schema := make([]models.SchemaField, 0, len(fieldMap))
	for name, ft := range fieldMap {
		schema = append(schema, models.SchemaField{
			Name:     name,
			Type:     ft,
			Required: true,
			Sample:   sampleMap[name],
		})
	}
	// fieldMap's own key order can't be preserved (it isn't the records'
	// original JSON key order — map decoding already lost that), and Go
	// randomizes map iteration order on every range, so without an explicit
	// sort this list would come back in a different order on every call.
	// This is now a user-facing preview the user confirms and that gets
	// stored verbatim, so a stable order matters here in a way it didn't
	// before.
	sort.Slice(schema, func(i, j int) bool { return schema[i].Name < schema[j].Name })
	return schema
}

func inferType(value string) models.FieldType {
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return models.FieldNumber
	}
	if looksLikeDate(value) {
		return models.FieldDate
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return models.FieldBoolean
	}
	return models.FieldString
}

// looksLikeDate is the shared date heuristic for both CSV/TXT/Excel string
// cells (via inferType) and JSON string values (via detectSchemaFromJSON) —
// JSON already tells number/bool apart by type, so only the date check needs
// to be shared.
func looksLikeDate(value string) bool {
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return true
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return true
	}
	return false
}

// DateFormatPresets maps a SchemaField.DateFormat key to the Go time
// layout it represents. Only these 4 keys are valid — see
// IsValidDateFormatPreset.
var DateFormatPresets = map[string]string{
	"DD/MM/YYYY": "02/01/2006",
	"MM/DD/YYYY": "01/02/2006",
	"DD-MM-YYYY": "02-01-2006",
	"YYYY/MM/DD": "2006/01/02",
}

// IsValidDateFormatPreset reports whether key is either "" (sin formato
// configurado, comportamiento por defecto) or a recognized entry of
// DateFormatPresets.
func IsValidDateFormatPreset(key string) bool {
	if key == "" {
		return true
	}
	_, ok := DateFormatPresets[key]
	return ok
}

// parseValue expects fieldName already resolved to its final schema field
// name (see dedupeFieldNames) — it matches schema entries exactly, without
// re-sanitizing, so it still finds duplicate-derived names like "928_2".
func parseValue(value string, schema []models.SchemaField, fieldName string) interface{} {
	for _, field := range schema {
		if field.Name == fieldName {
			switch field.Type {
			case models.FieldNumber:
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					if field.ImpliedDecimals > 0 {
						f = f / math.Pow(10, float64(field.ImpliedDecimals))
					}
					return f
				}
			case models.FieldBoolean:
				if b, err := strconv.ParseBool(value); err == nil {
					return b
				}
			case models.FieldDate:
				if field.DateFormat != "" {
					if t, err := time.Parse(DateFormatPresets[field.DateFormat], value); err == nil {
						return t
					}
					break
				}
				if t, err := time.Parse("2006-01-02", value); err == nil {
					return t
				}
				if t, err := time.Parse(time.RFC3339, value); err == nil {
					return t
				}
			}
		}
	}
	return value
}

func validateFieldValue(raw string, expected models.SchemaField) bool {
	if raw == "" {
		return true
	}
	switch expected.Type {
	case models.FieldNumber:
		if expected.ImpliedDecimals > 0 && strings.Contains(raw, ".") {
			return false
		}
		_, err := strconv.ParseFloat(raw, 64)
		return err == nil
	case models.FieldBoolean:
		_, err := strconv.ParseBool(raw)
		return err == nil
	case models.FieldDate:
		if expected.DateFormat != "" {
			_, err := time.Parse(DateFormatPresets[expected.DateFormat], raw)
			return err == nil
		}
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
				if !validateFieldValue(row[i], f) {
					return name, fmt.Sprintf("valor %q no es del tipo %s", row[i], f.Type)
				}
				break
			}
		}
	}
	return "", ""
}

func sanitizeFieldName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}

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

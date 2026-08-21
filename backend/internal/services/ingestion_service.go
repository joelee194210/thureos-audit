package services

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
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
}

func NewIngestionService(monitorRepo *repository.MonitorRepository) *IngestionService {
	return &IngestionService{monitorRepo: monitorRepo}
}

// IngestCSV parses a CSV file, detects schema, and stores data
func (s *IngestionService) IngestCSV(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, ',')
}

// IngestTXT parses a delimited text file (default: tab-separated), detects
// schema, and stores data. Same parser as IngestCSV — only the default
// delimiter differs, and SourceConfig.Delimiter always overrides either default.
func (s *IngestionService) IngestTXT(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	return s.ingestDelimited(ctx, monitor, file, '\t')
}

func (s *IngestionService) ingestDelimited(ctx context.Context, monitor *models.Monitor, file multipart.File, defaultDelimiter rune) (int, error) {
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
			return 0, fmt.Errorf("reading headers: %w", err)
		}
		headers = h
	}

	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("reading row: %w", err)
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

	if len(monitor.Schema) == 0 {
		monitor.Schema = detectSchemaFromCSV(headers, allRows)
	}

	documents := make([]interface{}, 0, len(allRows))
	for _, row := range allRows {
		doc := bson.M{"_ingested_at": time.Now()}
		for i, header := range headers {
			if i < len(row) {
				doc[sanitizeFieldName(header)] = parseValue(row[i], monitor.Schema, header)
			}
		}
		documents = append(documents, doc)
	}

	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return 0, fmt.Errorf("inserting data: %w", err)
	}

	now := time.Now()
	s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	})

	return count, nil
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
	s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	})

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
func (s *IngestionService) IngestExcel(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return 0, fmt.Errorf("opening Excel: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
	if monitor.SourceConfig != nil && monitor.SourceConfig.SheetName != "" {
		sheetName = monitor.SourceConfig.SheetName
	}
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, fmt.Errorf("reading Excel rows: %w", err)
	}

	if len(rows) < 2 {
		return 0, fmt.Errorf("excel file has no data rows")
	}

	headers := rows[0]
	dataRows := rows[1:]

	if len(monitor.Schema) == 0 {
		monitor.Schema = detectSchemaFromCSV(headers, dataRows)
	}

	documents := make([]interface{}, 0, len(dataRows))
	for _, row := range dataRows {
		doc := bson.M{"_ingested_at": time.Now()}
		for i, header := range headers {
			if i < len(row) {
				doc[sanitizeFieldName(header)] = parseValue(row[i], monitor.Schema, header)
			}
		}
		documents = append(documents, doc)
	}

	count, err := s.monitorRepo.InsertData(ctx, monitor.CollectionID, documents)
	if err != nil {
		return 0, fmt.Errorf("inserting data: %w", err)
	}

	now := time.Now()
	s.monitorRepo.Update(ctx, monitor.ID, bson.M{
		"schema":        monitor.Schema,
		"record_count":  monitor.RecordCount + int64(count),
		"last_ingested": now,
	})

	return count, nil
}

// Schema detection helpers

func detectSchemaFromCSV(headers []string, rows [][]string) []models.SchemaField {
	schema := make([]models.SchemaField, len(headers))
	for i, h := range headers {
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
			Name:     sanitizeFieldName(h),
			Type:     fieldType,
			Required: true,
			Sample:   sample,
		}
	}
	return schema
}

func detectSchemaFromJSON(records []map[string]interface{}) []models.SchemaField {
	fieldMap := make(map[string]models.FieldType)
	sampleMap := make(map[string]string)

	for _, record := range records {
		for key, val := range record {
			if _, exists := fieldMap[key]; !exists {
				switch val.(type) {
				case float64:
					fieldMap[key] = models.FieldNumber
				case bool:
					fieldMap[key] = models.FieldBoolean
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
	return schema
}

func inferType(value string) models.FieldType {
	if _, err := strconv.ParseFloat(value, 64); err == nil {
		return models.FieldNumber
	}
	if _, err := time.Parse("2006-01-02", value); err == nil {
		return models.FieldDate
	}
	if _, err := time.Parse(time.RFC3339, value); err == nil {
		return models.FieldDate
	}
	lower := strings.ToLower(value)
	if lower == "true" || lower == "false" {
		return models.FieldBoolean
	}
	return models.FieldString
}

func parseValue(value string, schema []models.SchemaField, fieldName string) interface{} {
	for _, field := range schema {
		if field.Name == sanitizeFieldName(fieldName) {
			switch field.Type {
			case models.FieldNumber:
				if f, err := strconv.ParseFloat(value, 64); err == nil {
					return f
				}
			case models.FieldBoolean:
				if b, err := strconv.ParseBool(value); err == nil {
					return b
				}
			case models.FieldDate:
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

func sanitizeFieldName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.ToLower(name)
	name = strings.ReplaceAll(name, " ", "_")
	name = strings.ReplaceAll(name, ".", "_")
	return name
}

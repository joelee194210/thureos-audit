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

	"github.com/joelee/datawatch/internal/models"
	"github.com/joelee/datawatch/internal/repository"
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
	reader := csv.NewReader(file)

	headers, err := reader.Read()
	if err != nil {
		return 0, fmt.Errorf("reading CSV headers: %w", err)
	}

	var allRows [][]string
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return 0, fmt.Errorf("reading CSV row: %w", err)
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

// IngestJSON parses a JSON array and stores data
func (s *IngestionService) IngestJSON(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return 0, fmt.Errorf("reading JSON: %w", err)
	}

	var records []map[string]interface{}
	if err := json.Unmarshal(data, &records); err != nil {
		var single map[string]interface{}
		if err2 := json.Unmarshal(data, &single); err2 != nil {
			return 0, fmt.Errorf("parsing JSON: %w", err)
		}
		records = []map[string]interface{}{single}
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

// IngestExcel parses an XLSX file and stores data
func (s *IngestionService) IngestExcel(ctx context.Context, monitor *models.Monitor, file multipart.File) (int, error) {
	f, err := excelize.OpenReader(file)
	if err != nil {
		return 0, fmt.Errorf("opening Excel: %w", err)
	}
	defer f.Close()

	sheetName := f.GetSheetName(0)
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

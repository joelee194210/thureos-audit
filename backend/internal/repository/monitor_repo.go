package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MonitorRepository struct {
	col *mongo.Collection
	db  *database.MongoDB
}

func NewMonitorRepository(db *database.MongoDB) *MonitorRepository {
	return &MonitorRepository{
		col: db.Collection("monitors"),
		db:  db,
	}
}

func (r *MonitorRepository) Create(ctx context.Context, monitor *models.Monitor) error {
	monitor.CreatedAt = time.Now()
	monitor.UpdatedAt = time.Now()

	result, err := r.col.InsertOne(ctx, monitor)
	if err != nil {
		return err
	}
	monitor.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *MonitorRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.Monitor, error) {
	var monitor models.Monitor
	err := r.col.FindOne(ctx, bson.M{"_id": id, "deleted_at": bson.M{"$exists": false}}).Decode(&monitor)
	if err != nil {
		return nil, err
	}
	return &monitor, nil
}

func (r *MonitorRepository) FindAll(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{"deleted_at": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

func (r *MonitorRepository) FindByOwner(ctx context.Context, ownerID primitive.ObjectID) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{"owner_id": ownerID, "deleted_at": bson.M{"$exists": false}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

func (r *MonitorRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

// Delete marca el monitor como borrado sin quitar nada. Su colección de
// datos, sus reglas, sus banderas rojas y su historial de cargas quedan
// intactos: en una plataforma de cumplimiento, borrar de verdad es perder la
// evidencia de algo que existió. Los cuatro finders excluyen los marcados, y
// por eso ningún llamador tuvo que cambiar.
func (r *MonitorRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{"deleted_at": now, "updated_at": now},
	})
	return err
}

// Restore deshace un borrado lógico. Sin esto, el borrado lógico sería
// esconder en vez de conservar: los datos siguen ahí pero solo se llega a
// ellos por la base.
func (r *MonitorRepository) Restore(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$unset": bson.M{"deleted_at": ""},
		"$set":   bson.M{"updated_at": time.Now()},
	})
	return err
}

// FindDeleted lista los monitores borrados, para poder restaurarlos.
func (r *MonitorRepository) FindDeleted(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{"deleted_at": bson.M{"$exists": true}})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	monitors := []models.Monitor{}
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

// FindPullMonitors returns all API-source monitors configured for pull mode.
// The caller filters by NextPullAt to decide which are actually due —
// same split as RuleRepository.FindScheduled + Scheduler.checkAndFireRules.
func (r *MonitorRepository) FindPullMonitors(ctx context.Context) ([]models.Monitor, error) {
	cursor, err := r.col.Find(ctx, bson.M{
		"source_type":        models.SourceAPI,
		"source_config.mode": models.APIModePull,
		"deleted_at":         bson.M{"$exists": false},
	})
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var monitors []models.Monitor
	if err := cursor.All(ctx, &monitors); err != nil {
		return nil, err
	}
	return monitors, nil
}

// GetDataCollection returns the dynamic collection for a monitor's data
func (r *MonitorRepository) GetDataCollection(collectionID string) *mongo.Collection {
	return r.db.Collection("data_" + collectionID)
}

func (r *MonitorRepository) InsertData(ctx context.Context, collectionID string, documents []interface{}) (int, error) {
	col := r.GetDataCollection(collectionID)
	result, err := col.InsertMany(ctx, documents)
	if err != nil {
		return 0, err
	}
	return len(result.InsertedIDs), nil
}

func (r *MonitorRepository) QueryData(ctx context.Context, collectionID string, filter bson.M, limit int64) ([]bson.M, error) {
	col := r.GetDataCollection(collectionID)

	findOpts := options.Find()
	if limit > 0 {
		findOpts.SetLimit(limit)
	}

	if filter == nil {
		filter = bson.M{}
	}

	cursor, err := col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

// QueryDataPaginated queries data with pagination and returns results + total count
func (r *MonitorRepository) QueryDataPaginated(ctx context.Context, collectionID string, filter bson.M, page, pageSize int64) ([]bson.M, int64, error) {
	col := r.GetDataCollection(collectionID)

	if filter == nil {
		filter = bson.M{}
	}

	total, err := col.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	skip := (page - 1) * pageSize
	findOpts := options.Find().SetSkip(skip).SetLimit(pageSize)

	cursor, err := col.Find(ctx, filter, findOpts)
	if err != nil {
		return nil, 0, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, 0, err
	}
	return results, total, nil
}

// IngestionEntry represents a single day's upload for a monitor
type IngestionEntry struct {
	MonitorID   string `json:"monitorId"`
	MonitorName string `json:"monitorName"`
	SourceType  string `json:"sourceType"`
	Date        string `json:"date"`
	RecordCount int64  `json:"recordCount"`
}

// GetIngestionHistory returns upload records grouped by date across all monitors
func (r *MonitorRepository) GetIngestionHistory(ctx context.Context) ([]IngestionEntry, error) {
	monitors, err := r.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	var entries []IngestionEntry
	for _, m := range monitors {
		col := r.GetDataCollection(m.CollectionID)

		pipeline := mongo.Pipeline{
			{{Key: "$group", Value: bson.D{
				{Key: "_id", Value: bson.D{{Key: "$dateToString", Value: bson.D{
					{Key: "format", Value: "%Y-%m-%d"},
					{Key: "date", Value: "$_ingested_at"},
				}}}},
				{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
			}}},
			{{Key: "$sort", Value: bson.D{{Key: "_id", Value: -1}}}},
		}

		cursor, err := col.Aggregate(ctx, pipeline)
		if err != nil {
			continue // skip monitors with no data
		}

		var results []bson.M
		if err := cursor.All(ctx, &results); err != nil {
			_ = cursor.Close(ctx)
			continue
		}
		_ = cursor.Close(ctx)

		for _, r := range results {
			date, _ := r["_id"].(string)
			count, _ := r["count"].(int32)
			if date == "" {
				continue
			}
			entries = append(entries, IngestionEntry{
				MonitorID:   m.ID.Hex(),
				MonitorName: m.Name,
				SourceType:  string(m.SourceType),
				Date:        date,
				RecordCount: int64(count),
			})
		}
	}

	if entries == nil {
		entries = []IngestionEntry{}
	}
	return entries, nil
}

// IterateData recorre todos los documentos de la colección de datos del
// monitor, aplicando fn a cada uno. Se usa para el backfill del timestamp
// derivado: cargar toda la colección en memoria no escala.
// backfillBatchSize acota cuántas escrituras se agrupan por viaje a Mongo.
const backfillBatchSize = 1000

// BackfillDataField completa targetField en los documentos que no lo tienen,
// calculando el valor con compute. Devuelve cuántos actualizó y cuántos no se
// pudieron construir (compute devolvió false) — esos se dejan como están: un
// valor inconstruible es dato incompleto, no dato inválido.
//
// El filtro por $exists corre del lado del servidor, así que los documentos ya
// procesados ni se traen: de ahí sale la idempotencia, y una segunda corrida
// no lee nada. Se proyectan solo los campos que compute necesita y las
// escrituras van en lotes, porque el caso de uso es recorrer la colección
// entera de un monitor.
func (r *MonitorRepository) BackfillDataField(
	ctx context.Context,
	collectionID string,
	targetField string,
	sourceFields []string,
	compute func(bson.M) (interface{}, bool),
) (int, int, error) {
	col := r.GetDataCollection(collectionID)

	projection := bson.D{{Key: "_id", Value: 1}}
	for _, f := range sourceFields {
		projection = append(projection, bson.E{Key: f, Value: 1})
	}

	cursor, err := col.Find(ctx,
		bson.M{targetField: bson.M{"$exists": false}},
		options.Find().SetProjection(projection),
	)
	if err != nil {
		return 0, 0, fmt.Errorf("finding data for backfill: %w", err)
	}
	defer func() { _ = cursor.Close(ctx) }()

	updated, skipped := 0, 0
	batch := make([]mongo.WriteModel, 0, backfillBatchSize)

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if _, err := col.BulkWrite(ctx, batch); err != nil {
			return fmt.Errorf("writing backfill batch: %w", err)
		}
		updated += len(batch)
		batch = batch[:0]
		return nil
	}

	for cursor.Next(ctx) {
		var doc bson.M
		if err := cursor.Decode(&doc); err != nil {
			return updated, skipped, fmt.Errorf("decoding document: %w", err)
		}
		value, ok := compute(doc)
		if !ok {
			skipped++
			continue
		}
		batch = append(batch, mongo.NewUpdateOneModel().
			SetFilter(bson.M{"_id": doc["_id"]}).
			SetUpdate(bson.M{"$set": bson.M{targetField: value}}))

		if len(batch) >= backfillBatchSize {
			if err := flush(); err != nil {
				return updated, skipped, err
			}
		}
	}
	if err := cursor.Err(); err != nil {
		return updated, skipped, fmt.Errorf("iterating data for backfill: %w", err)
	}
	if err := flush(); err != nil {
		return updated, skipped, err
	}
	return updated, skipped, nil
}

// rescaleExpression arma la conversión de un campo entre dos escalas de
// decimales implícitos. delta = decimalesNuevos - decimalesViejos:
//
//	delta > 0  el valor guardado se achica  → dividir por 10^delta
//	delta < 0  el valor guardado se agranda → multiplicar por 10^-delta
//
// Siempre por una potencia de diez ENTERA, nunca multiplicando por su
// recíproco: son montos, y `5000 * 0.1` en float64 da 500.00000000000006,
// mientras que `5000 / 10` da 500 exacto.
func rescaleExpression(field string, delta int) bson.D {
	if delta == 0 {
		return nil
	}

	op := "$divide"
	exp := delta
	if delta < 0 {
		op = "$multiply"
		exp = -delta
	}

	factor := int64(1)
	for i := 0; i < exp; i++ {
		factor *= 10
	}

	return bson.D{{Key: op, Value: bson.A{"$" + field, factor}}}
}

// RescaleDataField convierte un campo numérico entre dos escalas de
// decimales implícitos, en una sola operación del servidor.
//
// Solo toca documentos ingeridos ANTES de `cutoff` — estrictamente antes,
// `$lt` y no `$lte`: las fechas BSON tienen precisión de milisegundo, así
// que los nanosegundos de `time.Now()` se truncan al serializar, y un
// documento ingerido después de escribir el esquema (ya en escala nueva)
// pero dentro del mismo milisegundo que el cutoff serializaría igual a
// `_ingested_at == cutoff`. Con `$lte` ese documento se reescalaría dos
// veces; con `$lt` queda sin tocar, en la escala vieja — el modo de falla
// que el diseño prefiere (ver el comentario del llamador en UpdateSchema).
// Los posteriores al cutoff ya se guardaron con la configuración nueva y
// convertirlos otra vez los dejaría mal. El llamador toma el cutoff ANTES
// de escribir el esquema, de modo que la carrera que queda —una carga
// concurrente en la ventana entre ambos— deje una fila en la escala vieja,
// que se ve al mirar los datos, en vez de una convertida dos veces, que no
// se distingue de un monto legítimo.
//
// Devuelve (matched, skipped, error). matched es la cantidad de documentos
// en el alcance del cutoff con el campo numérico — el conteo correcto de
// "convertidos", a diferencia de ModifiedCount de Mongo: un valor guardado
// en 0 divide a 0, Mongo no lo cuenta como modificado, pero sí se convirtió.
// skipped es la cantidad de documentos en el mismo alcance de tiempo cuyo
// campo no es numérico (o no existe) — se dejan como están, y se devuelven
// aparte para que una conversión parcial se vea en vez de inferirse.
func (r *MonitorRepository) RescaleDataField(
	ctx context.Context,
	collectionID string,
	field string,
	delta int,
	cutoff time.Time,
) (int64, int64, error) {
	expr := rescaleExpression(field, delta)
	if expr == nil {
		return 0, 0, nil
	}

	col := r.GetDataCollection(collectionID)
	scope := bson.M{"_ingested_at": bson.M{"$lt": cutoff}}

	enAlcance, err := col.CountDocuments(ctx, scope)
	if err != nil {
		return 0, 0, fmt.Errorf("counting documents in scope for %s: %w", field, err)
	}

	res, err := col.UpdateMany(ctx,
		bson.M{
			field:          bson.M{"$type": "number"},
			"_ingested_at": bson.M{"$lt": cutoff},
		},
		mongo.Pipeline{
			{{Key: "$set", Value: bson.D{{Key: field, Value: expr}}}},
		},
	)
	if err != nil {
		return 0, 0, fmt.Errorf("rescaling %s: %w", field, err)
	}
	return res.MatchedCount, enAlcance - res.MatchedCount, nil
}

// EnsureDataIndex crea un índice sobre la colección de datos del monitor si
// todavía no existe (crear un índice ya existente es un no-op en MongoDB).
// Lo usan las reglas de velocidad: su pipeline ordena por agrupación y tiempo
// sobre toda la colección para poder formar pares que crucen lotes de
// ingesta, así que sin índice ese $sort escala mal.
func (r *MonitorRepository) EnsureDataIndex(ctx context.Context, collectionID string, keys bson.D) error {
	col := r.GetDataCollection(collectionID)
	if _, err := col.Indexes().CreateOne(ctx, mongo.IndexModel{Keys: keys}); err != nil {
		return fmt.Errorf("creating index on data_%s: %w", collectionID, err)
	}
	return nil
}

func (r *MonitorRepository) AggregateData(ctx context.Context, collectionID string, pipeline mongo.Pipeline) ([]bson.M, error) {
	col := r.GetDataCollection(collectionID)
	cursor, err := col.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer func() { _ = cursor.Close(ctx) }()

	var results []bson.M
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}
	return results, nil
}

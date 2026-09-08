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

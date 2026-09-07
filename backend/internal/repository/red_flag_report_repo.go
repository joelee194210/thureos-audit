package repository

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/thureos/compliance/internal/database"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/gridfs"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type redFlagReportMeta struct {
	ID           primitive.ObjectID `bson:"_id,omitempty"`
	RedFlagID    primitive.ObjectID `bson:"red_flag_id"`
	GeneratedAt  time.Time          `bson:"generated_at"`
	SHA256       string             `bson:"sha256"`
	GridFSFileID primitive.ObjectID `bson:"gridfs_file_id"`
	SizeBytes    int                `bson:"size_bytes"`
}

// RedFlagReportRepository guarda y recupera el PDF generado automáticamente
// para cada bandera roja: el binario vive en GridFS, la metadata (para
// trazabilidad — sha256, fecha, un documento por alerta) en una colección
// aparte.
type RedFlagReportRepository struct {
	meta   *mongo.Collection
	bucket *gridfs.Bucket
}

func NewRedFlagReportRepository(db *database.MongoDB) *RedFlagReportRepository {
	meta := db.Collection("red_flag_reports")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := meta.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "red_flag_id", Value: 1}},
		Options: options.Index().SetUnique(true),
	}); err != nil {
		fmt.Printf("WARNING: failed to create red_flag_reports index: %v\n", err)
	}

	bucket, err := gridfs.NewBucket(db.Database, options.GridFSBucket().SetName("red_flag_reports_files"))
	if err != nil {
		// No tumba el arranque del servidor — el reporte automático queda
		// inoperante y cada intento de uso lo loggea (ver Save/Get).
		fmt.Printf("WARNING: failed to open red_flag_reports_files GridFS bucket: %v\n", err)
		bucket = nil
	}

	return &RedFlagReportRepository{meta: meta, bucket: bucket}
}

// Save sube el PDF a GridFS y registra su metadata. Si ya existe un informe
// para este red flag (choca con el índice único), no es un error — un
// informe por alerta es la regla, no se regenera.
func (r *RedFlagReportRepository) Save(ctx context.Context, redFlagID primitive.ObjectID, pdf []byte) error {
	if r.bucket == nil {
		return fmt.Errorf("gridfs bucket not available")
	}
	sum := sha256.Sum256(pdf)

	fileID, err := r.bucket.UploadFromStream(
		fmt.Sprintf("red-flag-%s.pdf", redFlagID.Hex()),
		bytes.NewReader(pdf),
	)
	if err != nil {
		return fmt.Errorf("uploading report to gridfs: %w", err)
	}

	doc := redFlagReportMeta{
		RedFlagID:    redFlagID,
		GeneratedAt:  time.Now(),
		SHA256:       hex.EncodeToString(sum[:]),
		GridFSFileID: fileID,
		SizeBytes:    len(pdf),
	}
	if _, err := r.meta.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			// Ya había un informe — no se regenera. Se borra el archivo
			// recién subido para no dejar binarios huérfanos en GridFS.
			_ = r.bucket.Delete(fileID)
			return nil
		}
		return fmt.Errorf("saving report metadata: %w", err)
	}
	return nil
}

// Get busca el informe guardado de una bandera roja. Devuelve
// mongo.ErrNoDocuments si no existe ninguno.
func (r *RedFlagReportRepository) Get(ctx context.Context, redFlagID primitive.ObjectID) ([]byte, error) {
	if r.bucket == nil {
		return nil, mongo.ErrNoDocuments
	}
	var doc redFlagReportMeta
	if err := r.meta.FindOne(ctx, bson.M{"red_flag_id": redFlagID}).Decode(&doc); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if _, err := r.bucket.DownloadToStream(doc.GridFSFileID, &buf); err != nil {
		return nil, fmt.Errorf("downloading report from gridfs: %w", err)
	}
	return buf.Bytes(), nil
}

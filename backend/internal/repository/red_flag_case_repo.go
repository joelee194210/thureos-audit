package repository

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/thureos/compliance/internal/database"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// RedFlagCaseRepository gestiona la capa de caso de una red flag: el
// timeline inmutable de notas y las mutaciones de investigación
// (asignación, transiciones con disposición, SLA y prioridad).
type RedFlagCaseRepository struct {
	col      *mongo.Collection
	notesCol *mongo.Collection
}

func NewRedFlagCaseRepository(db *database.MongoDB) *RedFlagCaseRepository {
	r := &RedFlagCaseRepository{
		col:      db.Collection("red_flags"),
		notesCol: db.Collection("red_flag_notes"),
	}
	r.ensureIndexes()
	return r
}

func (r *RedFlagCaseRepository) ensureIndexes() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if _, err := r.notesCol.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{
			{Key: "red_flag_id", Value: 1},
			{Key: "created_at", Value: -1},
		},
	}); err != nil {
		log.Printf("red_flag_notes: creando índice: %v", err)
	}
}

// AddNote agrega una nota al timeline del caso. La colección no tiene
// Update ni Delete: en compliance el timeline es evidencia.
func (r *RedFlagCaseRepository) AddNote(ctx context.Context, note *models.RedFlagNote) error {
	if strings.TrimSpace(note.Text) == "" {
		return fmt.Errorf("la nota está vacía")
	}
	note.CreatedAt = time.Now()
	res, err := r.notesCol.InsertOne(ctx, note)
	if err != nil {
		return fmt.Errorf("insertando nota: %w", err)
	}
	note.ID = res.InsertedID.(primitive.ObjectID)
	return nil
}

// ListNotes devuelve las notas del caso, más reciente primero.
func (r *RedFlagCaseRepository) ListNotes(ctx context.Context, redFlagID primitive.ObjectID) ([]models.RedFlagNote, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	cur, err := r.notesCol.Find(ctx, bson.M{"red_flag_id": redFlagID},
		options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}}))
	if err != nil {
		return nil, fmt.Errorf("listando notas: %w", err)
	}
	var notes []models.RedFlagNote
	if err := cur.All(ctx, &notes); err != nil {
		return nil, fmt.Errorf("decodificando notas: %w", err)
	}
	return notes, nil
}

// Assign setea el asignado del caso, mueve el status a acknowledged y fija
// la prioridad (calculada por el caller desde la severidad). No toca
// sla_due_at: ese plazo se fija una sola vez, al crear el red flag — es el
// tiempo desde la primera detección, no desde que alguien lo toma.
func (r *RedFlagCaseRepository) Assign(ctx context.Context, id primitive.ObjectID, assignee primitive.ObjectID, priority int) error {
	update := bson.M{"$set": bson.M{
		"assignee_id": assignee,
		"status":      models.RedFlagAcknowledged,
		"priority":    priority,
		"updated_at":  time.Now(),
	}}
	return r.updateCase(ctx, id, update)
}

// Transition aplica un salto de estado validado, con los efectos del
// cierre (disposition, closed_at/by) cuando corresponde.
func (r *RedFlagCaseRepository) Transition(ctx context.Context, id primitive.ObjectID, to models.RedFlagStatus, disposition models.RedFlagDisposition, userID primitive.ObjectID) error {
	now := time.Now()
	set := bson.M{
		"status":     to,
		"updated_at": now,
	}
	// cierre = estados terminales; mismo criterio que services.ValidTransition
	if to == models.RedFlagResolved || to == models.RedFlagDismissed {
		set["disposition"] = disposition
		set["closed_at"] = now
		set["closed_by"] = userID
	}
	return r.updateCase(ctx, id, bson.M{"$set": set})
}

// updateCase es el único camino de escritura del caso: checkea que la
// red flag exista y no esté cerrada antes de aplicar el update.
func (r *RedFlagCaseRepository) updateCase(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	filter := bson.M{
		"_id":    id,
		"status": bson.M{"$nin": bson.A{models.RedFlagResolved, models.RedFlagDismissed}},
	}
	res, err := r.col.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("actualizando caso: %w", err)
	}
	if res.MatchedCount == 0 {
		// distinguir inexistente de cerrado para el status HTTP
		var probe models.RedFlag
		if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&probe); err != nil {
			return fmt.Errorf("caso %s no encontrado", id.Hex())
		}
		return ErrCaseClosed
	}
	return nil
}

// ErrCaseClosed: la red flag está en estado terminal (resolved/dismissed).
var ErrCaseClosed = fmt.Errorf("el caso ya está cerrado")

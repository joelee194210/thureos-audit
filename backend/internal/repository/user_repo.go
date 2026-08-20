package repository

import (
	"context"
	"time"

	"github.com/joelee/datawatch/internal/database"
	"github.com/joelee/datawatch/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type UserRepository struct {
	col *mongo.Collection
}

func NewUserRepository(db *database.MongoDB) *UserRepository {
	col := db.Collection("users")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	col.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "email", Value: 1}},
		Options: options.Index().SetUnique(true),
	})

	return &UserRepository{col: col}
}

func (r *UserRepository) Create(ctx context.Context, user *models.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()
	user.Active = true

	result, err := r.col.InsertOne(ctx, user)
	if err != nil {
		return err
	}
	user.ID = result.InsertedID.(primitive.ObjectID)
	return nil
}

func (r *UserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"email": email}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindByID(ctx context.Context, id primitive.ObjectID) (*models.User, error) {
	var user models.User
	err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user)
	if err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *UserRepository) FindAll(ctx context.Context) ([]models.User, error) {
	// Exclude soft-deleted users
	filter := bson.M{"deleted_at": bson.M{"$exists": false}}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var users []models.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

func (r *UserRepository) Update(ctx context.Context, id primitive.ObjectID, update bson.M) error {
	update["updated_at"] = time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{"$set": update})
	return err
}

// SoftDelete marks the user as deleted without removing the document.
// Regulatory retention requires keeping records for at least 5 years (FATF Rec. 11).
func (r *UserRepository) Delete(ctx context.Context, id primitive.ObjectID) error {
	now := time.Now()
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"deleted_at": now,
			"active":     false,
			"updated_at": now,
		},
	})
	return err
}

func (r *UserRepository) IncrementFailedLogin(ctx context.Context, id primitive.ObjectID, maxAttempts int, lockoutDuration time.Duration) error {
	update := bson.M{
		"$inc": bson.M{"failed_login_attempts": 1},
		"$set": bson.M{"updated_at": time.Now()},
	}

	if _, err := r.col.UpdateByID(ctx, id, update); err != nil {
		return err
	}

	// Check if threshold reached — lock the account
	var user models.User
	if err := r.col.FindOne(ctx, bson.M{"_id": id}).Decode(&user); err != nil {
		return err
	}
	// Value is already incremented by $inc above, so compare directly
	if user.FailedLoginAttempts >= maxAttempts {
		lockUntil := time.Now().Add(lockoutDuration)
		if _, err := r.col.UpdateByID(ctx, id, bson.M{
			"$set": bson.M{"locked_until": lockUntil},
		}); err != nil {
			return err
		}
	}
	return nil
}

func (r *UserRepository) ResetFailedLogin(ctx context.Context, id primitive.ObjectID) error {
	_, err := r.col.UpdateByID(ctx, id, bson.M{
		"$set": bson.M{
			"failed_login_attempts": 0,
			"updated_at":            time.Now(),
		},
		"$unset": bson.M{"locked_until": ""},
	})
	return err
}

// SeedAdmin ensures an admin user exists on startup.
// If a user with the given email already exists, it promotes them to admin.
// If no such user exists, it creates one with the given credentials.
// This is idempotent — safe to call on every startup.
func (r *UserRepository) SeedAdmin(ctx context.Context, email, hashedPassword, name string) error {
	// Check if any admin already exists
	count, err := r.col.CountDocuments(ctx, bson.M{"role": "admin", "deleted_at": bson.M{"$exists": false}})
	if err != nil {
		return err
	}
	if count > 0 {
		return nil // admin already exists, nothing to do
	}

	// Try to find existing user by email and promote
	now := time.Now()
	result, err := r.col.UpdateOne(ctx,
		bson.M{"email": email},
		bson.M{"$set": bson.M{"role": "admin", "active": true, "updated_at": now}},
	)
	if err != nil {
		return err
	}
	if result.MatchedCount > 0 {
		return nil // promoted existing user
	}

	// Create new admin user
	_, err = r.col.InsertOne(ctx, &models.User{
		Email:             email,
		Name:              name,
		Password:          hashedPassword,
		Role:              models.RoleAdmin,
		Active:            true,
		PasswordChangedAt: &now,
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	return err
}

func (r *UserRepository) MigrateRole(ctx context.Context, oldRole, newRole string) (int64, error) {
	result, err := r.col.UpdateMany(ctx,
		bson.M{"role": oldRole},
		bson.M{"$set": bson.M{"role": newRole, "updated_at": time.Now()}},
	)
	if err != nil {
		return 0, err
	}
	return result.ModifiedCount, nil
}

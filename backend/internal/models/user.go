package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Role string

const (
	RoleAdmin      Role = "admin"
	RoleCompliance Role = "compliance"
	RoleViewer     Role = "viewer"
)

type User struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email               string             `bson:"email" json:"email"`
	Name                string             `bson:"name" json:"name"`
	Password            string             `bson:"password" json:"-"`
	Role                Role               `bson:"role" json:"role"`
	Active              bool               `bson:"active" json:"active"`
	FailedLoginAttempts int                `bson:"failed_login_attempts" json:"-"`
	LockedUntil         *time.Time         `bson:"locked_until,omitempty" json:"-"`
	PasswordChangedAt   *time.Time         `bson:"password_changed_at,omitempty" json:"-"`
	DeletedAt           *time.Time         `bson:"deleted_at,omitempty" json:"deletedAt,omitempty"`
	CreatedAt           time.Time          `bson:"created_at" json:"createdAt"`
	UpdatedAt           time.Time          `bson:"updated_at" json:"updatedAt"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     Role   `json:"role"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

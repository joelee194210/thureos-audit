package services

import (
	"context"
	"fmt"
	"time"
	"unicode"

	"github.com/golang-jwt/jwt/v5"
	"github.com/joelee/datawatch/internal/config"
	"github.com/joelee/datawatch/internal/middleware"
	"github.com/joelee/datawatch/internal/models"
	"github.com/joelee/datawatch/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

const (
	MaxFailedAttempts = 5
	LockoutDuration   = 15 * time.Minute
	MinPasswordLength = 12
)

type AuthService struct {
	userRepo *repository.UserRepository
	cfg      *config.Config
}

func NewAuthService(userRepo *repository.UserRepository, cfg *config.Config) *AuthService {
	return &AuthService{userRepo: userRepo, cfg: cfg}
}

func ValidatePasswordComplexity(password string) error {
	if len(password) < MinPasswordLength {
		return fmt.Errorf("la contraseña debe tener al menos %d caracteres", MinPasswordLength)
	}
	var hasUpper, hasLower, hasDigit, hasSpecial bool
	for _, ch := range password {
		switch {
		case unicode.IsUpper(ch):
			hasUpper = true
		case unicode.IsLower(ch):
			hasLower = true
		case unicode.IsDigit(ch):
			hasDigit = true
		case unicode.IsPunct(ch) || unicode.IsSymbol(ch):
			hasSpecial = true
		}
	}
	if !hasUpper {
		return fmt.Errorf("la contraseña debe contener al menos una letra mayúscula")
	}
	if !hasLower {
		return fmt.Errorf("la contraseña debe contener al menos una letra minúscula")
	}
	if !hasDigit {
		return fmt.Errorf("la contraseña debe contener al menos un número")
	}
	if !hasSpecial {
		return fmt.Errorf("la contraseña debe contener al menos un carácter especial")
	}
	return nil
}

func (s *AuthService) Register(ctx context.Context, req models.RegisterRequest) (*models.AuthResponse, error) {
	if err := ValidatePasswordComplexity(req.Password); err != nil {
		return nil, err
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	// Self-registration always assigns viewer role.
	// Only admins can promote users via the user management API.
	now := time.Now()
	user := &models.User{
		Email:             req.Email,
		Name:              req.Name,
		Password:          string(hashed),
		Role:              models.RoleViewer,
		PasswordChangedAt: &now,
	}

	if err := s.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("creating user: %w", err)
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) Login(ctx context.Context, req models.LoginRequest) (*models.AuthResponse, error) {
	user, err := s.userRepo.FindByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if account is soft-deleted
	if user.DeletedAt != nil {
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check lockout — after too many failed attempts the account is temporarily locked
	if user.LockedUntil != nil && time.Now().Before(*user.LockedUntil) {
		return nil, fmt.Errorf("cuenta bloqueada temporalmente, intenta de nuevo después de %s",
			user.LockedUntil.Format("15:04"))
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		// Track failed attempt — log error but don't leak it to the caller
		if lockErr := s.userRepo.IncrementFailedLogin(ctx, user.ID, MaxFailedAttempts, LockoutDuration); lockErr != nil {
			fmt.Printf("WARNING: failed to increment login attempts for %s: %v\n", user.Email, lockErr)
		}
		return nil, fmt.Errorf("invalid credentials")
	}

	if !user.Active {
		return nil, fmt.Errorf("account is deactivated")
	}

	// Reset failed attempts on successful login
	if user.FailedLoginAttempts > 0 {
		if resetErr := s.userRepo.ResetFailedLogin(ctx, user.ID); resetErr != nil {
			fmt.Printf("WARNING: failed to reset login attempts for %s: %v\n", user.Email, resetErr)
		}
	}

	token, err := s.generateToken(user)
	if err != nil {
		return nil, fmt.Errorf("generating token: %w", err)
	}

	return &models.AuthResponse{Token: token, User: *user}, nil
}

func (s *AuthService) generateToken(user *models.User) (string, error) {
	claims := middleware.JWTClaims{
		UserID: user.ID.Hex(),
		Email:  user.Email,
		Role:   string(user.Role),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(s.cfg.SessionTimeout)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.JWTSecret))
}

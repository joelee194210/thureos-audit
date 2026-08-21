package services

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/thureos/compliance/internal/config"
	"github.com/thureos/compliance/internal/models"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
)

// fakeUserStore reproduce la lógica de umbral del repositorio real
// (UserRepository.IncrementFailedLogin) sin depender de Mongo, para que
// los tests observen el resultado completo del flujo y no un stub.
type fakeUserStore struct {
	user *models.User
}

func (f *fakeUserStore) FindByEmail(_ context.Context, email string) (*models.User, error) {
	if f.user == nil || f.user.Email != email {
		return nil, errNotFound
	}
	copia := *f.user
	return &copia, nil
}

func (f *fakeUserStore) Create(_ context.Context, u *models.User) error {
	f.user = u
	return nil
}

func (f *fakeUserStore) IncrementFailedLogin(_ context.Context, _ primitive.ObjectID, maxAttempts int, lockoutDuration time.Duration) error {
	f.user.FailedLoginAttempts++
	if f.user.FailedLoginAttempts >= maxAttempts {
		hasta := time.Now().Add(lockoutDuration)
		f.user.LockedUntil = &hasta
	}
	return nil
}

func (f *fakeUserStore) ResetFailedLogin(_ context.Context, _ primitive.ObjectID) error {
	f.user.FailedLoginAttempts = 0
	f.user.LockedUntil = nil
	return nil
}

var errNotFound = &notFoundError{}

type notFoundError struct{}

func (e *notFoundError) Error() string { return "not found" }

func usuarioDePrueba(t *testing.T, password string) *models.User {
	t.Helper()
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.MinCost)
	if err != nil {
		t.Fatalf("hashing password de prueba: %v", err)
	}
	return &models.User{
		ID:       primitive.NewObjectID(),
		Email:    "analista@thureos.local",
		Name:     "Analista",
		Password: string(hashed),
		Role:     models.RoleCompliance,
		Active:   true,
	}
}

func servicioDePrueba(store userStore) *AuthService {
	return NewAuthService(store, &config.Config{
		JWTSecret:      "test-secret",
		SessionTimeout: time.Hour,
	})
}

// Un bloqueo ya vencido debe devolverle al usuario sus intentos. Sin esto el
// contador sigue en el umbral y el próximo fallo dispara un bloqueo nuevo de
// inmediato, con lo que basta una petición cada LockoutDuration para dejar a
// una persona fuera del sistema indefinidamente.
func TestLogin_BloqueoVencidoDevuelveLosIntentos(t *testing.T) {
	store := &fakeUserStore{user: usuarioDePrueba(t, "Correcta-123!x")}
	vencido := time.Now().Add(-time.Minute)
	store.user.LockedUntil = &vencido
	store.user.FailedLoginAttempts = MaxFailedAttempts

	svc := servicioDePrueba(store)

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    store.user.Email,
		Password: "incorrecta",
	})
	if err == nil {
		t.Fatal("se esperaba que una contraseña incorrecta fuera rechazada")
	}

	if store.user.FailedLoginAttempts != 1 {
		t.Errorf("intentos fallidos = %d, se esperaba 1: el bloqueo vencido no devolvió los intentos",
			store.user.FailedLoginAttempts)
	}
	if store.user.LockedUntil != nil && time.Now().Before(*store.user.LockedUntil) {
		t.Error("la cuenta volvió a bloquearse tras un único fallo posterior a un bloqueo vencido")
	}
}

// Guarda del arreglo anterior: limpiar el bloqueo vencido no debe ablandar el
// bloqueo vigente. Mientras no venza, la contraseña correcta también se rechaza.
func TestLogin_BloqueoVigenteRechazaLaContrasenaCorrecta(t *testing.T) {
	store := &fakeUserStore{user: usuarioDePrueba(t, "Correcta-123!x")}
	vigente := time.Now().Add(10 * time.Minute)
	store.user.LockedUntil = &vigente
	store.user.FailedLoginAttempts = MaxFailedAttempts

	svc := servicioDePrueba(store)

	_, err := svc.Login(context.Background(), models.LoginRequest{
		Email:    store.user.Email,
		Password: "Correcta-123!x",
	})
	if err == nil {
		t.Fatal("una cuenta con bloqueo vigente no debe autenticar ni con la contraseña correcta")
	}
	if !strings.Contains(err.Error(), "bloqueada") {
		t.Errorf("error = %q, se esperaba el mensaje de cuenta bloqueada", err)
	}
	if store.user.FailedLoginAttempts != MaxFailedAttempts {
		t.Errorf("intentos = %d, se esperaba %d: el bloqueo vigente no debe alterar el contador",
			store.user.FailedLoginAttempts, MaxFailedAttempts)
	}
}

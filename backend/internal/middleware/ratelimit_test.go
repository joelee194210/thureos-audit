package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func appConLimitador(limitador fiber.Handler, status int) *fiber.App {
	app := fiber.New()
	app.Post("/", limitador, func(c *fiber.Ctx) error {
		return c.SendStatus(status)
	})
	return app
}

func statusDe(t *testing.T, app *fiber.App, n int) int {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil))
	if err != nil {
		t.Fatalf("petición %d: %v", n, err)
	}
	return resp.StatusCode
}

// El cupo del login existe para encarecer la fuerza bruta, no para racionar el
// trabajo de quien entra bien. Una oficina entera comparte una sola IP pública
// por NAT: si los inicios de sesión correctos consumieran cupo, diez analistas
// bastarían para dejar fuera al undécimo sin que nadie haya hecho nada malo.
func TestLoginRateLimiter_LosIniciosCorrectosNoConsumenCupo(t *testing.T) {
	app := appConLimitador(LoginRateLimiter(), fiber.StatusOK)

	for i := 1; i <= LoginMaxAttempts*2; i++ {
		if got := statusDe(t, app, i); got != fiber.StatusOK {
			t.Fatalf("petición %d: status = %d, un inicio de sesión correcto no debe consumir cupo", i, got)
		}
	}
}

// Los intentos fallidos sí se cuentan: en eso consiste el límite.
func TestLoginRateLimiter_LosIntentosFallidosAgotanElCupo(t *testing.T) {
	app := appConLimitador(LoginRateLimiter(), fiber.StatusUnauthorized)

	for i := 1; i <= LoginMaxAttempts; i++ {
		if got := statusDe(t, app, i); got != fiber.StatusUnauthorized {
			t.Fatalf("petición %d dentro del cupo: status = %d, se esperaba 401", i, got)
		}
	}
	if got := statusDe(t, app, LoginMaxAttempts+1); got != fiber.StatusTooManyRequests {
		t.Errorf("petición %d: status = %d, se esperaba 429", LoginMaxAttempts+1, got)
	}
}

// En el registro el cupo cuenta también las altas correctas: lo que se frena
// aquí es justamente la creación masiva de cuentas, que devuelve 201.
func TestRegisterRateLimiter_LasAltasCorrectasConsumenCupo(t *testing.T) {
	app := appConLimitador(RegisterRateLimiter(), fiber.StatusCreated)

	for i := 1; i <= RegisterMaxAttempts; i++ {
		if got := statusDe(t, app, i); got != fiber.StatusCreated {
			t.Fatalf("petición %d dentro del cupo: status = %d, se esperaba 201", i, got)
		}
	}
	if got := statusDe(t, app, RegisterMaxAttempts+1); got != fiber.StatusTooManyRequests {
		t.Errorf("petición %d: status = %d, se esperaba 429", RegisterMaxAttempts+1, got)
	}
}

func TestRateLimiter_El429TraeMensaje(t *testing.T) {
	app := appConLimitador(RegisterRateLimiter(), fiber.StatusCreated)
	for i := 1; i <= RegisterMaxAttempts; i++ {
		statusDe(t, app, i)
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/", nil))
	if err != nil {
		t.Fatalf("petición sobrante: %v", err)
	}
	var cuerpo map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&cuerpo); err != nil {
		t.Fatalf("decodificando el cuerpo del 429: %v", err)
	}
	if cuerpo["error"] == "" {
		t.Error("el 429 debe traer un mensaje en el campo \"error\"")
	}
}

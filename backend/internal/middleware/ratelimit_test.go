package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v2"
)

// El limitador debe dejar pasar exactamente el cupo y rechazar el siguiente
// intento con 429, para que la fuerza bruta contra /auth/login tenga coste.
func TestAuthRateLimiter_RechazaAlSuperarElCupo(t *testing.T) {
	const cupo = 3

	app := fiber.New()
	app.Post("/login", AuthRateLimiter(cupo, time.Minute), func(c *fiber.Ctx) error {
		return c.SendStatus(fiber.StatusOK)
	})

	for i := 1; i <= cupo; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/login", nil))
		if err != nil {
			t.Fatalf("petición %d: %v", i, err)
		}
		if resp.StatusCode != fiber.StatusOK {
			t.Fatalf("petición %d dentro del cupo: status = %d, se esperaba 200", i, resp.StatusCode)
		}
	}

	resp, err := app.Test(httptest.NewRequest(http.MethodPost, "/login", nil))
	if err != nil {
		t.Fatalf("petición %d: %v", cupo+1, err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Fatalf("petición %d: status = %d, se esperaba 429", cupo+1, resp.StatusCode)
	}

	var cuerpo map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&cuerpo); err != nil {
		t.Fatalf("decodificando el cuerpo del 429: %v", err)
	}
	if cuerpo["error"] == "" {
		t.Error("el 429 debe traer un mensaje en el campo \"error\"")
	}
}

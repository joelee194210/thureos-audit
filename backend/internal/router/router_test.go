package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
	"github.com/thureos/compliance/internal/config"
	"github.com/thureos/compliance/internal/middleware"
)

// Los handlers van en cero: estas pruebas sólo comprueban que el limitador
// está cableado delante de las rutas públicas de autenticación. Lo que pasa
// por detrás lo absorbe el middleware recover que Setup ya instala; lo único
// que importa aquí es que la petición sobrante reciba 429 y no llegue nunca
// al handler.
func appDePrueba() *fiber.App {
	app := fiber.New()
	Setup(app, &config.Config{}, &Handlers{})
	return app
}

func peticionesHasta(t *testing.T, app *fiber.App, ruta string, n int) {
	t.Helper()
	for i := 1; i <= n; i++ {
		resp, err := app.Test(httptest.NewRequest(http.MethodPost, ruta, nil))
		if err != nil {
			t.Fatalf("%s petición %d: %v", ruta, i, err)
		}
		if resp.StatusCode == fiber.StatusTooManyRequests {
			t.Fatalf("%s: 429 en la petición %d, dentro del cupo de %d", ruta, i, n)
		}
	}
}

func exigir429(t *testing.T, app *fiber.App, ruta string) {
	t.Helper()
	resp, err := app.Test(httptest.NewRequest(http.MethodPost, ruta, nil))
	if err != nil {
		t.Fatalf("%s petición sobrante: %v", ruta, err)
	}
	if resp.StatusCode != fiber.StatusTooManyRequests {
		t.Errorf("%s: status = %d al superar el cupo, se esperaba 429", ruta, resp.StatusCode)
	}
}

func TestSetup_LoginLimitadoPorIP(t *testing.T) {
	app := appDePrueba()
	peticionesHasta(t, app, "/api/v1/auth/login", middleware.LoginMaxAttempts)
	exigir429(t, app, "/api/v1/auth/login")
}

func TestSetup_RegistroLimitadoPorIP(t *testing.T) {
	app := appDePrueba()
	peticionesHasta(t, app, "/api/v1/auth/register", middleware.RegisterMaxAttempts)
	exigir429(t, app, "/api/v1/auth/register")
}

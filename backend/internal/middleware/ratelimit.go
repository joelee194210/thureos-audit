package middleware

import (
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/limiter"
)

// Cupos por IP para los endpoints públicos de autenticación. El de login deja
// margen a quien se equivoca al teclear sin abrir la puerta a la fuerza bruta;
// el de registro corta la creación masiva de cuentas.
const (
	LoginMaxAttempts    = 10
	LoginWindow         = 5 * time.Minute
	RegisterMaxAttempts = 5
	RegisterWindow      = time.Hour
)

// AuthRateLimiter limita por IP las peticiones a un endpoint de autenticación.
//
// El almacén es el de memoria que trae Fiber, no Redis: Redis es opcional en
// esta aplicación —el servidor arranca igual si falla— y un limitador que
// dependiera de él podría dejar el login inaccesible. La contrapartida es que
// el cupo es por proceso: detrás de más de una instancia hay que revisarlo.
//
// La clave es c.IP(), que devuelve el par del socket porque fiber.Config no
// declara proxies de confianza. Correcto en un solo host; detrás de un proxy
// inverso todos los usuarios compartirían cupo y habría que configurar
// TrustedProxies antes que nada.
func AuthRateLimiter(max int, window time.Duration) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        max,
		Expiration: window,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "demasiados intentos, espera unos minutos antes de volver a probar",
			})
		},
	})
}

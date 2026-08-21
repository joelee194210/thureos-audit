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

// authRateLimiter limita por IP las peticiones a un endpoint de autenticación.
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
// LoginRateLimiter limita por IP los intentos *fallidos* de inicio de sesión.
// Los correctos no consumen cupo: una oficina entera comparte una sola IP
// pública por NAT, y racionar el trabajo legítimo no encarece la fuerza bruta.
func LoginRateLimiter() fiber.Handler {
	return authRateLimiter(LoginMaxAttempts, LoginWindow, true)
}

// RegisterRateLimiter limita por IP las altas, contando también las correctas:
// lo que se frena aquí es precisamente la creación masiva de cuentas.
func RegisterRateLimiter() fiber.Handler {
	return authRateLimiter(RegisterMaxAttempts, RegisterWindow, false)
}

func authRateLimiter(max int, window time.Duration, skipSuccessful bool) fiber.Handler {
	return limiter.New(limiter.Config{
		Max:                    max,
		Expiration:             window,
		SkipSuccessfulRequests: skipSuccessful,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "demasiados intentos, espera unos minutos antes de volver a probar",
			})
		},
	})
}

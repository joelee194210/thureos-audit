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

// Cupo del endpoint público de ingesta push. No hay objetivo de fuerza
// bruta aquí (el token tiene 256 bits de entropía, es inviable de
// adivinar) — esto acota el radio de disponibilidad de un endpoint
// público sin límite de frecuencia propio.
const (
	IngestMaxRequests = 60
	IngestWindow      = time.Minute
)

// IngestRateLimiter limita por IP las peticiones al endpoint de ingesta
// push, para que no sea un vector de agotamiento de recursos sin cota.
func IngestRateLimiter() fiber.Handler {
	return limiter.New(limiter.Config{
		Max:        IngestMaxRequests,
		Expiration: IngestWindow,
		LimitReached: func(c *fiber.Ctx) error {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{
				"error": "too many requests",
			})
		},
	})
}

// IngestBodySizeLimit rejects requests whose Content-Length exceeds
// maxBytes before IngestJSON parses the body or any Mongo work happens. It
// does NOT prevent the allocation itself: this app does not set
// StreamRequestBody, so fasthttp has already buffered the full body (up to
// the app-wide 50MB BodyLimit, see cmd/server/main.go) before any
// middleware runs. It also can't see a chunked request that omits
// Content-Length. Defense in depth for a public endpoint — not a hard
// memory bound.
func IngestBodySizeLimit(maxBytes int) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Request().Header.ContentLength() > maxBytes {
			return c.Status(fiber.StatusRequestEntityTooLarge).JSON(fiber.Map{
				"error": "request body too large",
			})
		}
		return c.Next()
	}
}

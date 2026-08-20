package config

import (
	"log"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	Port           string
	AppEnv         string
	MongoURI       string
	RedisURL       string
	JWTSecret      string
	AnthropicKey   string
	AllowedOrigins string
	SessionTimeout time.Duration
	AdminEmail     string
	AdminPassword  string
}

func Load() *Config {
	_ = godotenv.Load()
	appEnv := getEnv("APP_ENV", "development")
	return &Config{
		Port:           getEnv("PORT", "8080"),
		AppEnv:         appEnv,
		MongoURI:       getEnv("MONGO_URI", "mongodb://localhost:27017/datawatch"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      loadJWTSecret(appEnv),
		AnthropicKey:   getEnv("ANTHROPIC_API_KEY", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		SessionTimeout: getEnvDuration("SESSION_TIMEOUT_HOURS", 8),
		AdminEmail:     getEnv("ADMIN_EMAIL", ""),
		AdminPassword:  getEnv("ADMIN_PASSWORD", ""),
	}
}

// loadJWTSecret evita el peor fallo silencioso posible: arrancar en
// producción firmando tokens con un secreto que está en el repositorio.
func loadJWTSecret(appEnv string) string {
	if v := os.Getenv("JWT_SECRET"); v != "" {
		return v
	}
	if appEnv == "production" {
		log.Fatal("FATAL: JWT_SECRET no está definida y APP_ENV=production. " +
			"Defina JWT_SECRET con un valor secreto antes de arrancar.")
	}
	log.Println("ADVERTENCIA: JWT_SECRET no está definida. Usando el secreto de " +
		"desarrollo, que es público. No usar fuera de desarrollo local.")
	return "dev-secret-change-me"
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func getEnvDuration(key string, defaultHours int) time.Duration {
	if v := os.Getenv(key); v != "" {
		if hours, err := strconv.Atoi(v); err == nil && hours > 0 {
			return time.Duration(hours) * time.Hour
		}
		log.Printf("WARNING: invalid value %q for %s, using default %dh", v, key, defaultHours)
	}
	return time.Duration(defaultHours) * time.Hour
}

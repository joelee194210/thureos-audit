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
	return &Config{
		Port:           getEnv("PORT", "8080"),
		MongoURI:       getEnv("MONGO_URI", "mongodb://localhost:27017/datawatch"),
		RedisURL:       getEnv("REDIS_URL", "redis://localhost:6379"),
		JWTSecret:      getEnv("JWT_SECRET", "dev-secret-change-me"),
		AnthropicKey:   getEnv("ANTHROPIC_API_KEY", ""),
		AllowedOrigins: getEnv("ALLOWED_ORIGINS", "http://localhost:3000"),
		SessionTimeout: getEnvDuration("SESSION_TIMEOUT_HOURS", 8),
		AdminEmail:     getEnv("ADMIN_EMAIL", ""),
		AdminPassword:  getEnv("ADMIN_PASSWORD", ""),
	}
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

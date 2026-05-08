package config

import "os"

// Config holds all application configuration, populated from environment variables
type Config struct {
	// Server
	Port string

	// JWT
	JWTSecret string

	// Redis
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisEnabled  bool

	// PostgreSQL
	PostgresDSN     string
	PostgresEnabled bool

	// NATS
	NatsURL     string
	NatsEnabled bool

	// Binance
	BinanceEnabled bool
}

// Load reads configuration from environment variables with sensible defaults
func Load() *Config {
	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		JWTSecret: getEnv("JWT_SECRET", "ybtech-mini-exchange-secret-key-2026"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisEnabled:  getEnv("REDIS_ENABLED", "") == "true",

		PostgresDSN:     getEnv("POSTGRES_DSN", "postgres://postgres:postgres@localhost:5432/mini_exchange?sslmode=disable"),
		PostgresEnabled: getEnv("POSTGRES_ENABLED", "") == "true",

		NatsURL:     getEnv("NATS_URL", "nats://localhost:4222"),
		NatsEnabled: getEnv("NATS_ENABLED", "") == "true",

		BinanceEnabled: getEnv("BINANCE_ENABLED", "") == "true",
	}

	return cfg
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

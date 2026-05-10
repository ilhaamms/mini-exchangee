package config

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func loadDotEnv(filename string) {
	f, err := os.Open(filename)
	if err != nil {
		return 
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])
		
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}

	if err := scanner.Err(); err != nil {
		log.Printf("WARNING: error reading .env: %v", err)
	}
}

type Config struct {
	
	Port string

	
	JWTSecret string

	
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisEnabled  bool

	
	PostgresHost     string
	PostgresPort     string
	PostgresUser     string
	PostgresPassword string
	PostgresDB       string
	PostgresSSLMode  string
	PostgresDSN      string 
	PostgresEnabled  bool

	
	NatsURL     string
	NatsEnabled bool

	
	BinanceEnabled bool
}

func Load() *Config {
	loadDotEnv(".env")

	pgHost := getEnv("POSTGRES_HOST", "localhost")
	pgPort := getEnv("POSTGRES_PORT", "5432")
	pgUser := getEnv("POSTGRES_USER", "postgres")
	pgPass := getEnv("POSTGRES_PASSWORD", "postgres")
	pgDB := getEnv("POSTGRES_DB", "mini_exchange")
	pgSSL := getEnv("POSTGRES_SSLMODE", "disable")

	cfg := &Config{
		Port:      getEnv("PORT", "8080"),
		JWTSecret: getEnv("JWT_SECRET", "ybtech-mini-exchange-secret-key-2026"),

		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisEnabled:  getEnv("REDIS_ENABLED", "") == "true",

		PostgresHost:     pgHost,
		PostgresPort:     pgPort,
		PostgresUser:     pgUser,
		PostgresPassword: pgPass,
		PostgresDB:       pgDB,
		PostgresSSLMode:  pgSSL,
		PostgresDSN: "host=" + pgHost +
			" port=" + pgPort +
			" user=" + pgUser +
			" password=" + pgPass +
			" dbname=" + pgDB +
			" sslmode=" + pgSSL,
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

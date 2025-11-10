package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	// Common
	Env      string
	LogLevel string
	// API
	Port        string
	Storage     string
	DatabaseURL string
	// Provider
	Provider        string
	ExchangeAPIBase string
	ExchangeAPIKey  string
	// Worker
	WorkerType      string
	WorkerPoll      time.Duration
	WorkerBatchSize int
	// gRPC
	GRPCAddr       string
	GRPCTarget     string
	RequestTimeout time.Duration
	// Redis (idempotency)
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	RedisTTL      time.Duration
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func atoiDef(s string, def int) int {
	i, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return i
}

// Load reads environment variables and applies defaults.
func Load() Config {
	return Config{
		Env:             getEnv("ENV", "local"),
		LogLevel:        getEnv("LOG_LEVEL", "info"),
		Port:            getEnv("PORT", "8080"),
		Storage:         getEnv("STORAGE", "pg"),
		DatabaseURL:     getEnv("DATABASE_URL", ""),
		Provider:        getEnv("PROVIDER", "fake"),
		ExchangeAPIBase: getEnv("EXCHANGE_API_BASE", "https://api.exchangeratesapi.io"),
		ExchangeAPIKey:  getEnv("EXCHANGE_API_KEY", ""),
		WorkerType:      getEnv("WORKER_TYPE", "db"),
		WorkerPoll:      time.Duration(atoiDef(getEnv("WORKER_POLL_MS", "250"), 250)) * time.Millisecond,
		WorkerBatchSize: atoiDef(getEnv("WORKER_BATCH_LIMIT", "10"), 10),
		GRPCAddr:        getEnv("GRPC_ADDR", ":9090"),
		GRPCTarget:      getEnv("GRPC_TARGET", "localhost:9090"),
		RequestTimeout:  time.Duration(atoiDef(getEnv("REQUEST_TIMEOUT_MS", "3000"), 3000)) * time.Millisecond,
		RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:   getEnv("REDIS_PASSWORD", ""),
		RedisDB:         atoiDef(getEnv("REDIS_DB", "0"), 0),
		RedisTTL:        time.Duration(atoiDef(getEnv("IDEMPOTENCY_TTL_MS", "86400000"), 86400000)) * time.Millisecond,
	}
}

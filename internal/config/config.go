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
	DatabaseURL string
	// HTTP server
	ShutdownTimeout time.Duration
	// Provider
	Provider        string
	ExchangeAPIBase string
	ExchangeAPIKey  string
	// HTTP backoff for provider calls (milliseconds)
	HTTPBackoffInitial time.Duration
	HTTPBackoffMax     time.Duration
	HTTPBackoffTotal   time.Duration
	// PG pool sizing
	PGMaxConns int
	PGMinConns int
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
	// Chan worker
	ChanQueueSize   int
	ChanConcurrency int
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
		Env:                getEnv("ENV", "local"),
		LogLevel:           getEnv("LOG_LEVEL", "info"),
		Port:               getEnv("PORT", "8080"),
		DatabaseURL:        getEnv("DATABASE_URL", ""),
		ShutdownTimeout:    time.Duration(atoiDef(getEnv("SHUTDOWN_TIMEOUT_MS", "10000"), 10000)) * time.Millisecond,
		Provider:           getEnv("PROVIDER", "fake"),
		ExchangeAPIBase:    getEnv("EXCHANGE_API_BASE", "https://api.exchangeratesapi.io"),
		ExchangeAPIKey:     getEnv("EXCHANGE_API_KEY", ""),
		HTTPBackoffInitial: time.Duration(atoiDef(getEnv("HTTP_BACKOFF_INITIAL_MS", "200"), 200)) * time.Millisecond,
		HTTPBackoffMax:     time.Duration(atoiDef(getEnv("HTTP_BACKOFF_MAX_MS", "1000"), 1000)) * time.Millisecond,
		HTTPBackoffTotal:   time.Duration(atoiDef(getEnv("HTTP_BACKOFF_TOTAL_MS", "3000"), 3000)) * time.Millisecond,
		PGMaxConns:         atoiDef(getEnv("PG_MAX_CONNS", "5"), 5),
		PGMinConns:         atoiDef(getEnv("PG_MIN_CONNS", "1"), 1),
		WorkerType:         getEnv("WORKER_TYPE", "db"),
		WorkerPoll:         time.Duration(atoiDef(getEnv("WORKER_POLL_MS", "250"), 250)) * time.Millisecond,
		WorkerBatchSize:    atoiDef(getEnv("WORKER_BATCH_LIMIT", "10"), 10),
		GRPCAddr:           getEnv("GRPC_ADDR", ":9090"),
		GRPCTarget:         getEnv("GRPC_TARGET", "dns:///worker:9090"),
		RequestTimeout:     time.Duration(atoiDef(getEnv("REQUEST_TIMEOUT_MS", "3000"), 3000)) * time.Millisecond,
		RedisAddr:          getEnv("REDIS_ADDR", "redis:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		RedisDB:            atoiDef(getEnv("REDIS_DB", "0"), 0),
		RedisTTL:           time.Duration(atoiDef(getEnv("IDEMPOTENCY_TTL_MS", "86400000"), 86400000)) * time.Millisecond,
		ChanQueueSize:      atoiDef(getEnv("CHAN_QUEUE_SIZE", "100"), 100),
		ChanConcurrency:    atoiDef(getEnv("CHAN_CONCURRENCY", "2"), 2),
	}
}

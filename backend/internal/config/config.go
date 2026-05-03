package config

import (
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	ServerAddr    string
	PostgresDSN   string
	MongoURI      string
	MongoDatabase string
	RedisAddr     string
	RedisPassword string
	WorkerCount   int
	BufferSize    int
	RateLimit     float64 // requests/sec
	RateBurst     int
}

func Load() *Config {
	_ = godotenv.Load()

	return &Config{
		ServerAddr:    getEnv("SERVER_ADDR", ":8080"),
		PostgresDSN:   getEnv("POSTGRES_DSN", "postgres://ims:ims@localhost:5432/ims?sslmode=disable"),
		MongoURI:      getEnv("MONGO_URI", "mongodb://localhost:27017"),
		MongoDatabase: getEnv("MONGO_DATABASE", "ims"),
		RedisAddr:     getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		WorkerCount:   getEnvInt("WORKER_COUNT", 20),
		BufferSize:    getEnvInt("BUFFER_SIZE", 100000),
		RateLimit:     getEnvFloat("RATE_LIMIT", 2000),
		RateBurst:     getEnvInt("RATE_BURST", 5000),
	}
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvFloat(key string, defaultVal float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

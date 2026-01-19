package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds all configuration for the application
type Config struct {
	App      AppConfig
	Database DatabaseConfig
	Worker   WorkerConfig
}

// AppConfig holds application configuration
type AppConfig struct {
	Env  string
	Port string
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host            string
	Port            string
	User            string
	Password        string
	Name            string
	PoolMax         int
	PoolMinConns    int
	PoolMaxConnLife time.Duration
}

// WorkerConfig holds worker configuration
type WorkerConfig struct {
	Count     int
	BatchSize int
}

// Load loads configuration from environment variables
func Load() *Config {
	return &Config{
		App: AppConfig{
			Env:  getEnv("APP_ENV", "development"),
			Port: getEnv("APP_PORT", "8080"),
		},
		Database: DatabaseConfig{
			Host:            getEnv("DB_HOST", "localhost"),
			Port:            getEnv("DB_PORT", "5432"),
			User:            getEnv("DB_USER", "postgres"),
			Password:        getEnv("DB_PASSWORD", "postgres"),
			Name:            getEnv("DB_NAME", "costing"),
			PoolMax:         getEnvInt("DB_POOL_MAX", 50),
			PoolMinConns:    getEnvInt("DB_POOL_MIN", 10),
			PoolMaxConnLife: time.Duration(getEnvInt("DB_POOL_MAX_CONN_LIFE_MINUTES", 30)) * time.Minute,
		},
		Worker: WorkerConfig{
			Count:     getEnvInt("WORKER_COUNT", 100),
			BatchSize: getEnvInt("BATCH_SIZE", 1000),
		},
	}
}

// DSN returns the database connection string
func (c *DatabaseConfig) DSN() string {
	return "postgres://" + c.User + ":" + c.Password + "@" + c.Host + ":" + c.Port + "/" + c.Name + "?sslmode=disable"
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

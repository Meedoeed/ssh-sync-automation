package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerCfg
	Database DatabaseCfg
	Sync     SyncCfg
	Log      LogCfg
}

type ServerCfg struct {
	Port string
}

type DatabaseCfg struct {
	Host     string
	Port     string
	User     string
	Password string
	DBName   string
	SSLMode  string
}

type SyncCfg struct {
	Interval      time.Duration
	RetryMaxAtmpt int
	RetryDelay    time.Duration
	SSHConTimeout time.Duration
	SSHKeepAlive  time.Duration
}

type LogCfg struct {
	Level string
}

func Load() *Config {
	return &Config{
		Server: ServerCfg{
			Port: getEnv("SERVER_PORT", "8080"),
		},
		Database: DatabaseCfg{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", ""),
			DBName:   getEnv("DB_NAME", "ssh_sync"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},
		Sync: SyncCfg{
			Interval:      getEnvDuration("SYNC_INTERVAL", 5*time.Minute),
			RetryMaxAtmpt: getEnvInt("RETRY_MAX_ATTEMPTS", 5),
			RetryDelay:    getEnvDuration("RETRY_DELAY", 5*time.Second),
			SSHConTimeout: getEnvDuration("SSH_CONNECT_TIMEOUT", 10*time.Second),
			SSHKeepAlive:  getEnvDuration("SSH_KEEPALIVE", 30*time.Second),
		},
		Log: LogCfg{
			Level: getEnv("LOG_LEVEL", "info"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}

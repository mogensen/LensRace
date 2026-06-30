// Package config loads runtime configuration from the environment.
package config

import "os"

// Config holds the settings needed to start the server.
type Config struct {
	Port   string
	DBPath string
}

// Load reads configuration from the environment, falling back to
// development-friendly defaults.
func Load() Config {
	return Config{
		Port:   getEnv("PORT", "3000"),
		DBPath: getEnv("DB_PATH", "lensrace.db"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

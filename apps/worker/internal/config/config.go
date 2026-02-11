package config

import "os"

type Config struct {
	Port          string
	DBPath        string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8787"),
		DBPath:        getEnv("DB_PATH", "./data/wheel.db"),
		JWTSecret:     getEnv("JWT_SECRET", "change-me-in-production"),
		AdminUsername: getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword: getEnv("ADMIN_PASSWORD", "admin"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

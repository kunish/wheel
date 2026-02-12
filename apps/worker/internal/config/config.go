package config

import "os"

type Config struct {
	Port          string
	DataPath      string
	JWTSecret     string
	AdminUsername string
	AdminPassword string
}

func Load() *Config {
	return &Config{
		Port:          getEnv("PORT", "8787"),
		DataPath:      getEnv("DATA_PATH", "./data"),
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

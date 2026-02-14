package config

import (
	"os"
	"strings"
)

type Config struct {
	Port          string
	DataPath      string
	JWTSecret     string
	AdminUsername string
	AdminPassword string

	// WebSocket
	AllowedOrigins []string // empty = dev mode (allow localhost)

	// Observability
	MetricsEnabled  bool
	OtelEnabled     bool
	OtelEndpoint    string
	OtelServiceName string
}

func Load() *Config {
	return &Config{
		Port:            getEnv("PORT", "8787"),
		DataPath:        getEnv("DATA_PATH", "./data"),
		JWTSecret:       getEnv("JWT_SECRET", "change-me-in-production"),
		AdminUsername:   getEnv("ADMIN_USERNAME", "admin"),
		AdminPassword:   getEnv("ADMIN_PASSWORD", "admin"),
		AllowedOrigins:  parseOrigins(os.Getenv("ALLOWED_ORIGINS")),
		MetricsEnabled:  getEnv("METRICS_ENABLED", "") == "true",
		OtelEnabled:     getEnv("OTEL_ENABLED", "") == "true",
		OtelEndpoint:    getEnv("OTEL_EXPORTER_ENDPOINT", "localhost:4317"),
		OtelServiceName: getEnv("OTEL_SERVICE_NAME", "wheel-gateway"),
	}
}

func parseOrigins(s string) []string {
	if s == "" {
		return nil
	}
	var origins []string
	for _, o := range strings.Split(s, ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			origins = append(origins, o)
		}
	}
	return origins
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

package config

import "os"

type Config struct {
	Port          string
	DataPath      string
	JWTSecret     string
	AdminUsername string
	AdminPassword string

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
		MetricsEnabled:  getEnv("METRICS_ENABLED", "") == "true",
		OtelEnabled:     getEnv("OTEL_ENABLED", "") == "true",
		OtelEndpoint:    getEnv("OTEL_EXPORTER_ENDPOINT", "localhost:4317"),
		OtelServiceName: getEnv("OTEL_SERVICE_NAME", "wheel-gateway"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

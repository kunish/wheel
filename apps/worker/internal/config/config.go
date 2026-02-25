package config

import (
	"log"
	"os"
)

// Version is set at build time via -ldflags.
var Version = "dev"

type Config struct {
	Port  string
	DBDSN string
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
		Port:  getEnv("PORT", "8787"),
		DBDSN:           getEnv("DB_DSN", "root:@tcp(127.0.0.1:4000)/wheel?parseTime=true&charset=utf8mb4"),
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

// Validate checks for insecure default credentials and logs warnings.
// It does not prevent startup, but makes the warnings hard to miss.
func (c *Config) Validate() {
	insecureSecrets := []string{"change-me-in-production", "dummy"}
	for _, s := range insecureSecrets {
		if c.JWTSecret == s {
			log.Println("========================================")
			log.Println("[SECURITY WARNING] JWT_SECRET is using a default/insecure value. Set JWT_SECRET env var in production!")
			log.Println("========================================")
			break
		}
	}
	if c.AdminPassword == "admin" {
		log.Println("========================================")
		log.Println("[SECURITY WARNING] ADMIN_PASSWORD is using the default value 'admin'. Set ADMIN_PASSWORD env var in production!")
		log.Println("========================================")
	}
}

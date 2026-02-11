package main

import (
	"context"
	"log"
	"path/filepath"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/handler"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/ws"
)

func main() {
	cfg := config.Load()

	// ── Security warnings ──
	if cfg.JWTSecret == "change-me-in-production" {
		log.Println("[SECURITY WARNING] JWT_SECRET is using the default value. Set JWT_SECRET env var in production!")
	}
	if cfg.AdminPassword == "admin" {
		log.Println("[SECURITY WARNING] ADMIN_PASSWORD is using the default value. Set ADMIN_PASSWORD env var in production!")
	}

	// ── Database ──
	dbPath := filepath.Join(cfg.DataPath, "wheel.db")
	database, err := db.Open(dbPath)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	// Run Drizzle-compatible migrations
	migrationsDir := filepath.Join(cfg.DataPath, "..", "drizzle")
	if err := db.Migrate(database.DB, migrationsDir); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// ── Log Database (separate file for write-heavy logs) ──
	logDBPath := filepath.Join(cfg.DataPath, "wheel-logs.db")
	logDatabase, err := db.OpenLogDB(logDBPath)
	if err != nil {
		log.Fatalf("Failed to open log database: %v", err)
	}
	defer logDatabase.Close()

	if err := db.MigrateLogDB(logDatabase.DB); err != nil {
		log.Fatalf("Failed to migrate log database: %v", err)
	}

	// ── Cache ──
	kv := cache.New()
	defer kv.Close()

	// ── WebSocket Hub ──
	hub := ws.New()

	// ── Handlers ──
	h := &handler.Handler{
		DB:     database,
		LogDB:  logDatabase,
		Cache:  kv,
		Config: cfg,
	}

	rh := &handler.RelayHandler{
		DB:            database,
		LogDB:         logDatabase,
		Cache:         kv,
		Broadcast:     hub.Broadcast,
		StreamTracker: hub,
	}

	// ── Router ──
	r := gin.Default()
	h.RegisterRoutes(r)
	rh.RegisterRelayRoutes(r)

	// WebSocket endpoint
	r.GET("/api/v1/ws", hub.HandleWS)

	// ── Cron Jobs ──
	c := cron.New()
	// Sync model prices and channel models every 6 hours
	c.AddFunc("0 */6 * * *", func() {
		log.Println("[cron] Syncing model prices from models.dev...")
		if _, err := handler.SyncPricesFromModelsDev(context.Background(), database); err != nil {
			log.Printf("[cron] price sync error: %v", err)
		}
	})
	c.AddFunc("0 */6 * * *", func() {
		log.Println("[cron] Syncing all channel models...")
		result, err := relay.SyncAllModels(context.Background(), database, kv)
		if err != nil {
			log.Printf("[cron] model sync error: %v", err)
		} else {
			log.Printf("[cron] model sync done: %d channels synced, %d errors",
				result.SyncedChannels, len(result.Errors))
			hub.Broadcast("model-sync", result)
		}
	})
	c.Start()
	defer c.Stop()

	// ── Start Server ──
	addr := ":" + cfg.Port
	log.Printf("Wheel worker listening on %s", addr)
	if err := r.Run(addr); err != nil {
		log.Fatalf("Server error: %v", err)
	}
}

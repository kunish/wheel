// @title Wheel API
// @version 1.0.0
// @description LLM API Gateway — aggregate providers, manage load balancing, and track usage/costs.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/bifrostx"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/handler"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/seed"
	"github.com/kunish/wheel/apps/worker/internal/service"
	"github.com/kunish/wheel/apps/worker/internal/ws"
	"github.com/robfig/cron/v3"
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

	// ── Seed subcommand ──
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		if err := seed.Run(context.Background(), database, logDatabase); err != nil {
			log.Fatalf("Seed failed: %v", err)
		}
		return
	}

	// ── Cache ──
	kv := cache.New()
	defer kv.Close()

	// ── WebSocket Hub ──
	hub := ws.New(cfg.JWTSecret, cfg.AllowedOrigins)

	// ── Observability (must be before LogWriter so it can record drop metrics) ──
	obs, err := observe.New(cfg.MetricsEnabled, cfg.OtelEnabled, cfg.OtelEndpoint, cfg.OtelServiceName)
	if err != nil {
		log.Fatalf("Failed to initialize observability: %v", err)
	}
	if obs != nil {
		defer obs.Shutdown(context.Background())
	}

	// ── Relay Managers ──
	cbm := relay.NewCircuitBreakerManager(obs, kv)
	sm := relay.NewSessionManager()
	bal := relay.NewBalancerState()

	// ── LogWriter (batched async log persistence) ──
	logWriter := db.NewLogWriter(logDatabase, database, hub.Broadcast, hub, obs, kv)

	// Use a cancellable context for background services
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logWriterDone := make(chan struct{})
	go func() {
		logWriter.Run(ctx)
		close(logWriterDone)
	}()

	// ── Background Log Cleanup ──
	db.StartLogCleanup(ctx, database, logDatabase)
	cbm.StartCleanup(ctx)
	sm.StartCleanup(ctx)

	// ── Handlers ──
	h := &handler.Handler{
		DB:     database,
		LogDB:  logDatabase,
		Cache:  kv,
		Config: cfg,
	}

	rh := &handler.RelayHandler{
		Handler: handler.Handler{
			DB:     database,
			LogDB:  logDatabase,
			Cache:  kv,
			Config: cfg,
		},
		Broadcast:       hub.Broadcast,
		StreamTracker:   hub,
		LogWriter:       logWriter,
		Observer:        obs,
		CircuitBreakers: cbm,
		Sessions:        sm,
		Balancer:        bal,
	}

	bifrostClient, err := bifrostx.New(ctx, database, cfg.BifrostDebugRaw)
	if err != nil {
		log.Fatalf("Failed to initialize bifrost executor: %v", err)
	}
	rh.Bifrost = bifrostClient

	// ── Router ──
	r := gin.Default()
	h.RegisterRoutes(r)
	rh.RegisterRelayRoutes(r)

	// Prometheus metrics endpoint
	if metricsHandler := obs.MetricsHandler(); metricsHandler != nil {
		r.GET("/metrics", gin.WrapH(metricsHandler))
	}

	// WebSocket endpoint
	r.GET("/api/v1/ws", hub.HandleWS)

	// ── Cron Jobs ──
	c := cron.New()
	// Sync model prices and channel models every 6 hours
	c.AddFunc("0 */6 * * *", func() {
		log.Println("[cron] Syncing model prices from models.dev...")
		if _, err := service.SyncPricesFromModelsDev(context.Background(), database); err != nil {
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

	// ── Start Server with Graceful Shutdown ──
	addr := ":" + cfg.Port
	srv := &http.Server{Addr: addr, Handler: r}

	go func() {
		log.Printf("Wheel worker listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	log.Println("[shutdown] Signal received, shutting down...")

	// Gracefully shut down HTTP server (stop accepting new requests)
	if err := srv.Shutdown(context.Background()); err != nil {
		log.Printf("[shutdown] HTTP server shutdown error: %v", err)
	}

	// Wait for LogWriter to flush remaining buffered logs
	<-logWriterDone
	log.Println("[shutdown] All logs flushed, exiting.")
}

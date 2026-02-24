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
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/robfig/cron/v3"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/handler"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/seed"
	"github.com/kunish/wheel/apps/worker/internal/service"
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
	database, err := db.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// ── Seed subcommand ──
	if len(os.Args) > 1 && os.Args[1] == "seed" {
		if err := seed.Run(context.Background(), database); err != nil {
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

	// ── HTTP Clients (with timeout) ──
	nonStreamClient := &http.Client{Timeout: 120 * time.Second}
	streamClient := &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{Timeout: 30 * time.Second}).DialContext,
		},
		// No overall timeout — streams can run indefinitely
	}

	// ── LogWriter (batched async log persistence) ──
	logWriter := db.NewLogWriter(database, hub.Broadcast, hub, obs, kv)

	// Use a cancellable context for background services
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	logWriterDone := make(chan struct{})
	go func() {
		logWriter.Run(ctx)
		close(logWriterDone)
	}()

	// ── Background Log Cleanup ──
	db.StartLogCleanup(ctx, database)
	cbm.StartCleanup(ctx)
	sm.StartCleanup(ctx)

	// ── Handlers ──
	h := &handler.Handler{
		DB:              database,
		Cache:           kv,
		Config:          cfg,
		CircuitBreakers: cbm,
	}

	rh := &handler.RelayHandler{
		Handler: handler.Handler{
			DB:     database,
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
		HTTPClient:      nonStreamClient,
		StreamClient:    streamClient,
	}

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

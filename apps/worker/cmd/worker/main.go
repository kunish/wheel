// @title Wheel API
// @version 1.0.0
// @description LLM API Gateway — aggregate providers, manage load balancing, and track usage/costs.
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/handler"
	mcpgw "github.com/kunish/wheel/apps/worker/internal/mcp"
	"github.com/kunish/wheel/apps/worker/internal/observe"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/seed"
	"github.com/kunish/wheel/apps/worker/internal/service"
	"github.com/kunish/wheel/apps/worker/internal/ws"
	"github.com/robfig/cron/v3"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	cfg := config.Load()
	cfg.Validate()

	// ── Database ──
	database, err := db.Open(cfg.DBDSN)
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	if err := db.Migrate(database.DB); err != nil {
		log.Fatalf("Failed to run migrations: %v", err)
	}

	// ── Ensure a default API key exists for new installations ──
	dal.EnsureDefaultApiKey(context.Background(), database)
	if err := codexruntime.MaterializeAuthFiles(context.Background(), database); err != nil {
		log.Fatalf("Failed to materialize managed Codex auth files: %v", err)
	}

	// ── Cache (created early so startup can populate it) ──
	kv := cache.New()
	defer kv.Close()

	// Fetch model metadata and create builtin profiles FIRST
	var startupMetadata map[string]service.ModelMeta
	if metadata, err := service.FetchAndFlattenMetadata(); err != nil {
		log.Printf("[startup] Failed to fetch model metadata: %v", err)
	} else {
		startupMetadata = metadata
		kv.Put(service.MetadataKVKey, metadata, service.MetadataTTL)
		log.Printf("[startup] Synced %d model metadata entries", len(metadata))
	}
	if startupMetadata != nil {
		service.UpsertBuiltinProfilesFromMetadata(context.Background(), database, startupMetadata)
	} else {
		service.UpsertBuiltinProfiles(context.Background(), database)
	}

	// One-time fix: reset groups incorrectly assigned by earlier buggy logic
	dal.ResetGroupProfilesOnce(context.Background(), database)

	// Now assign orphaned groups (builtin profiles already exist)
	if dpID, err := dal.EnsureDefaultProfile(context.Background(), database); err == nil && dpID > 0 {
		_ = dal.AssignOrphanedGroups(context.Background(), database, dpID)
	}

	// ── CLI subcommands ──
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "seed":
			if err := seed.Run(context.Background(), database); err != nil {
				log.Fatalf("Seed failed: %v", err)
			}
			return
		case "reset-password":
			resetPassword(context.Background(), database)
			return
		}
	}

	// ── WebSocket Hub ──
	hub := ws.New(cfg.JWTSecret)

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
	batchStore := relay.NewBatchStore()
	asyncStore := relay.NewAsyncStore()

	// ── Routing Engine (load rules from DB) ──
	routingEngine := relay.NewRoutingEngine()
	if rules, err := dal.ListRoutingRules(context.Background(), database); err == nil {
		routingEngine.LoadFromModels(rules)
		log.Printf("[startup] Loaded %d routing rules", len(rules))
	} else {
		log.Printf("[startup] Failed to load routing rules: %v", err)
	}

	// ── Health Checker (probe channels every 60s) ──
	healthChecker := relay.NewHealthChecker(60 * time.Second)

	// ── Rate Limit Plugin ──
	rateLimitPlugin := relay.NewRateLimitPlugin(func(ctx *relay.RelayContext) relay.RateLimitConfig {
		rpm, _ := ctx.GinCtx.Get("rpmLimit")
		tpm, _ := ctx.GinCtx.Get("tpmLimit")
		rpmVal, _ := rpm.(int)
		tpmVal, _ := tpm.(int)
		return relay.RateLimitConfig{RPM: rpmVal, TPM: tpmVal}
	})

	// ── Plugin Pipeline ──
	plugins := relay.NewPluginPipeline(rateLimitPlugin)

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

	codexResult, err := initEmbeddedCodexRuntime(ctx, cfg, log.Default(), func(cfg *config.Config) (codexRuntimeService, error) {
		return codexruntime.NewFromConfig(cfg)
	})
	if err != nil {
		log.Fatalf("[startup] embedded Codex runtime failed: %v", err)
	}
	if codexResult != nil && codexResult.errCh != nil {
		go func() {
			err := <-codexResult.errCh
			if err == nil || errors.Is(err, context.Canceled) {
				return
			}
			log.Printf("[codex-runtime] embedded service exited with error, shutting down worker: %v", err)
			stop()
		}()
	}

	// Create in-process HTTP clients for Codex channels when handler is available.
	var codexStreamClient, codexHTTPClient *http.Client
	if codexResult != nil && codexResult.handler != nil {
		codexTransport := handler.NewInMemoryTransport(codexResult.handler)
		codexStreamClient = &http.Client{Transport: codexTransport}
		codexHTTPClient = &http.Client{
			Transport: codexTransport,
			Timeout:   120 * time.Second,
		}
		log.Println("[startup] Codex runtime handler-only mode active — no port 8317")
	}

	logWriterDone := make(chan struct{})
	go func() {
		logWriter.Run(ctx)
		close(logWriterDone)
	}()

	// ── Background Log Cleanup ──
	db.StartLogCleanup(ctx, database)
	cbm.StartCleanup(ctx)
	sm.StartCleanup(ctx)
	rateLimitPlugin.StartCleanup(ctx.Done())
	go func() {
		ticker := time.NewTicker(30 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				batchStore.Cleanup(24 * time.Hour)
				asyncStore.Cleanup(24 * time.Hour)
			}
		}
	}()

	// ── Start Health Check Loop ──
	healthChecker.Start(ctx, func() []relay.HealthCheckTarget {
		channels, err := dal.ListChannels(context.Background(), database)
		if err != nil {
			log.Printf("[healthcheck] Failed to list channels: %v", err)
			return nil
		}
		var targets []relay.HealthCheckTarget
		for _, ch := range channels {
			if !ch.Enabled || len(ch.BaseUrls) == 0 {
				continue
			}
			url := ch.BaseUrls[0].URL
			if url == "" {
				continue
			}
			headers := make(map[string]string)
			if len(ch.Keys) > 0 && ch.Keys[0].ChannelKey != "" {
				headers["Authorization"] = "Bearer " + ch.Keys[0].ChannelKey
			}
			targets = append(targets, relay.HealthCheckTarget{
				ChannelID: ch.ID,
				URL:       url + "/v1/models",
				Headers:   headers,
			})
		}
		return targets
	})

	// ── MCP Gateway Manager ──
	mcpManager := mcpgw.NewManager()
	if err := dal.BackfillMCPToolsToExecuteAllowAll(context.Background(), database); err != nil {
		log.Printf("[startup] MCP tools_to_execute backfill failed: %v", err)
	}
	if mcpClients, err := dal.ListMCPClients(context.Background(), database); err == nil {
		for i := range mcpClients {
			if !mcpClients[i].Enabled {
				continue
			}
			if err := mcpManager.AddClient(context.Background(), &mcpClients[i]); err != nil {
				log.Printf("[startup] MCP client %q connect failed: %v", mcpClients[i].Name, err)
			}
		}
		log.Printf("[startup] Loaded %d MCP clients", len(mcpClients))
	} else {
		log.Printf("[startup] Failed to load MCP clients: %v", err)
	}
	mcpManager.StartToolSync(ctx)

	// ── MCP Server (expose aggregated tools) ──
	mcpSrv := mcpgw.NewServer(mcpManager)
	mcpSrv.SyncTools()
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				mcpSrv.SyncTools()
			}
		}
	}()

	// ── Handlers ──
	dlock := db.NewDistributedLock(database)

	h := &handler.Handler{
		DB:                    database,
		Cache:                 kv,
		Config:                cfg,
		CircuitBreakers:       cbm,
		DLock:                 dlock,
		CodexManagementClient: codexHTTPClient,
	}

	rh := &handler.RelayHandler{
		Handler: handler.Handler{
			DB:                    database,
			Cache:                 kv,
			Config:                cfg,
			CircuitBreakers:       cbm,
			DLock:                 dlock,
			CodexManagementClient: codexHTTPClient,
		},
		Broadcast:         hub.Broadcast,
		StreamTracker:     hub,
		LogWriter:         logWriter,
		Observer:          obs,
		CircuitBreakers:   cbm,
		Sessions:          sm,
		Balancer:          bal,
		HTTPClient:        nonStreamClient,
		StreamClient:      streamClient,
		Plugins:           plugins,
		RoutingEngine:     routingEngine,
		HealthChecker:     healthChecker,
		MCPManager:        mcpManager,
		MCPServer:         mcpSrv,
		BatchStore:        batchStore,
		AsyncStore:        asyncStore,
		CopilotRelay:      handler.NewCopilotRelay(database),
		CodexStreamClient: codexStreamClient,
		CodexHTTPClient:   codexHTTPClient,
	}

	// ── Router ──
	r := gin.Default()
	h.RegisterRoutes(r)
	rh.RegisterRelayRoutes(r)
	rh.RegisterRelayAdminRoutes(r)

	// Prometheus metrics endpoint
	if metricsHandler := obs.MetricsHandler(); metricsHandler != nil {
		r.GET("/metrics", gin.WrapH(metricsHandler))
	}

	// WebSocket endpoint
	r.GET("/api/v1/ws", hub.HandleWS)

	// ── Cron Jobs ──
	c := cron.New()
	// Sync model prices and builtin profiles every 6 hours
	_, _ = c.AddFunc("0 */6 * * *", func() {
		log.Println("[cron] Syncing from models.dev...")
		if _, err := service.SyncAllFromModelsDev(context.Background(), database); err != nil {
			log.Printf("[cron] models.dev sync error: %v", err)
		}
	})
	_, _ = c.AddFunc("0 */6 * * *", func() {
		log.Println("[cron] Refreshing model metadata...")
		if metadata, err := service.FetchAndFlattenMetadata(); err != nil {
			log.Printf("[cron] metadata refresh error: %v", err)
		} else {
			kv.Put(service.MetadataKVKey, metadata, service.MetadataTTL)
			service.UpsertBuiltinProfilesFromMetadata(context.Background(), database, metadata)
			log.Printf("[cron] metadata refreshed: %d entries", len(metadata))
		}
	})
	_, _ = c.AddFunc("0 */6 * * *", func() {
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

	// Stop MCP Manager
	mcpManager.Stop()

	// Wait for LogWriter to flush remaining buffered logs
	<-logWriterDone
	log.Println("[shutdown] All logs flushed, exiting.")
}

func resetPassword(ctx context.Context, database *bun.DB) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	username := fs.String("u", "", "Admin username")
	password := fs.String("p", "", "Admin password")
	fs.Parse(os.Args[2:])

	if *username == "" || *password == "" {
		fmt.Println("Usage: wheel reset-password -u <username> -p <password>")
		os.Exit(1)
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	user, err := dal.GetUser(ctx, database)
	if err != nil {
		log.Fatalf("Failed to query user: %v", err)
	}

	if user == nil {
		if _, err := dal.CreateUser(ctx, database, *username, string(hashed)); err != nil {
			log.Fatalf("Failed to create user: %v", err)
		}
		log.Printf("Admin user created: %s", *username)
	} else {
		if err := dal.UpdateUsername(ctx, database, user.ID, *username); err != nil {
			log.Fatalf("Failed to update username: %v", err)
		}
		if err := dal.UpdatePassword(ctx, database, user.ID, string(hashed)); err != nil {
			log.Fatalf("Failed to update password: %v", err)
		}
		log.Printf("Admin user updated: %s", *username)
	}
}

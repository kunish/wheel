package seed

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

const seedMarkerKey = "__seed_data_loaded"

// Run populates both databases with demo data. It is idempotent —
// repeated calls skip seeding if the marker setting already exists.
func Run(ctx context.Context, db *bun.DB, logDB *bun.DB) error {
	// Idempotency check
	var count int
	count, err := db.NewSelect().TableExpr("settings").
		Where("key = ?", seedMarkerKey).Count(ctx)
	if err != nil {
		return fmt.Errorf("check seed marker: %w", err)
	}
	if count > 0 {
		log.Println("[seed] Seed data already loaded, skipping")
		return nil
	}

	log.Println("[seed] Seeding demo data...")

	if err := seedChannels(ctx, db); err != nil {
		return fmt.Errorf("seed channels: %w", err)
	}
	if err := seedGroups(ctx, db); err != nil {
		return fmt.Errorf("seed groups: %w", err)
	}
	if err := seedAPIKeys(ctx, db); err != nil {
		return fmt.Errorf("seed api keys: %w", err)
	}
	if err := seedPricing(ctx, db); err != nil {
		return fmt.Errorf("seed pricing: %w", err)
	}
	if err := seedLogs(ctx, logDB); err != nil {
		return fmt.Errorf("seed logs: %w", err)
	}

	// Write marker
	marker := &types.Setting{Key: seedMarkerKey, Value: "true"}
	if _, err := db.NewInsert().Model(marker).Exec(ctx); err != nil {
		return fmt.Errorf("write seed marker: %w", err)
	}

	log.Println("[seed] Demo data seeded successfully")
	return nil
}

func seedChannels(ctx context.Context, db *bun.DB) error {
	channels := []types.Channel{
		{
			Name:    "OpenAI",
			Type:    types.OutboundOpenAI,
			Enabled: true,
			BaseUrls: types.BaseUrlList{
				{URL: "https://api.openai.com", Delay: 0},
			},
			Model:    types.StringList{"gpt-4o", "gpt-4o-mini", "gpt-4.1", "gpt-4.1-mini", "gpt-4.1-nano", "o3-mini"},
			AutoSync: true,
			Order:    0,
		},
		{
			Name:    "Anthropic",
			Type:    types.OutboundAnthropic,
			Enabled: true,
			BaseUrls: types.BaseUrlList{
				{URL: "https://api.anthropic.com", Delay: 0},
			},
			Model:    types.StringList{"claude-sonnet-4-20250514", "claude-haiku-4-20250514", "claude-3.5-sonnet-20241022"},
			AutoSync: true,
			Order:    1,
		},
		{
			Name:    "Google Gemini",
			Type:    types.OutboundGemini,
			Enabled: true,
			BaseUrls: types.BaseUrlList{
				{URL: "https://generativelanguage.googleapis.com", Delay: 0},
			},
			Model:    types.StringList{"gemini-2.5-pro", "gemini-2.5-flash", "gemini-2.0-flash"},
			AutoSync: false,
			Order:    2,
		},
	}

	for i := range channels {
		if _, err := db.NewInsert().Model(&channels[i]).Exec(ctx); err != nil {
			return err
		}
		// Add a demo key per channel
		ck := &types.ChannelKey{
			ChannelID:  channels[i].ID,
			ChannelKey: fmt.Sprintf("sk-demo-%s-key-000000000000", []string{"openai", "anthropic", "gemini"}[i]),
			Enabled:    true,
			Remark:     "Demo key",
		}
		if _, err := db.NewInsert().Model(ck).Exec(ctx); err != nil {
			return err
		}
	}

	log.Printf("[seed] Created %d channels", len(channels))
	return nil
}

func seedGroups(ctx context.Context, db *bun.DB) error {
	// Fetch channels to reference their IDs
	var channels []types.Channel
	if err := db.NewSelect().Model(&channels).OrderExpr("id ASC").Scan(ctx); err != nil {
		return err
	}
	if len(channels) < 3 {
		return fmt.Errorf("expected at least 3 channels, got %d", len(channels))
	}

	openaiID := channels[0].ID
	anthropicID := channels[1].ID
	geminiID := channels[2].ID

	groups := []struct {
		group types.Group
		items []types.GroupItem
	}{
		{
			group: types.Group{
				Name:              "GPT Models",
				Mode:              types.GroupModeFailover,
				FirstTokenTimeOut: 15000,
				SessionKeepTime:   300000,
				Order:             0,
			},
			items: []types.GroupItem{
				{ChannelID: openaiID, ModelName: "gpt-4o", Priority: 1, Weight: 100, Enabled: true},
				{ChannelID: openaiID, ModelName: "gpt-4o-mini", Priority: 1, Weight: 100, Enabled: true},
				{ChannelID: openaiID, ModelName: "gpt-4.1", Priority: 1, Weight: 100, Enabled: true},
				{ChannelID: openaiID, ModelName: "gpt-4.1-mini", Priority: 1, Weight: 100, Enabled: true},
			},
		},
		{
			group: types.Group{
				Name:              "Claude Models",
				Mode:              types.GroupModeWeighted,
				FirstTokenTimeOut: 20000,
				SessionKeepTime:   600000,
				Order:             1,
			},
			items: []types.GroupItem{
				{ChannelID: anthropicID, ModelName: "claude-sonnet-4-20250514", Priority: 1, Weight: 70, Enabled: true},
				{ChannelID: anthropicID, ModelName: "claude-haiku-4-20250514", Priority: 1, Weight: 30, Enabled: true},
			},
		},
		{
			group: types.Group{
				Name:              "Gemini Models",
				Mode:              types.GroupModeRoundRobin,
				FirstTokenTimeOut: 10000,
				Order:             2,
			},
			items: []types.GroupItem{
				{ChannelID: geminiID, ModelName: "gemini-2.5-pro", Priority: 1, Weight: 50, Enabled: true},
				{ChannelID: geminiID, ModelName: "gemini-2.5-flash", Priority: 1, Weight: 50, Enabled: true},
			},
		},
	}

	for _, g := range groups {
		grp := g.group
		if _, err := db.NewInsert().Model(&grp).Exec(ctx); err != nil {
			return err
		}
		for _, item := range g.items {
			item.GroupID = grp.ID
			if _, err := db.NewInsert().Model(&item).Exec(ctx); err != nil {
				return err
			}
		}
	}

	log.Printf("[seed] Created %d groups", len(groups))
	return nil
}

func seedAPIKeys(ctx context.Context, db *bun.DB) error {
	now := time.Now()
	keys := []types.APIKey{
		{
			Name:            "Production Key",
			APIKey:          "sk-wheel-demo-prod-aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			Enabled:         true,
			ExpireAt:        now.AddDate(1, 0, 0).Unix(),
			MaxCost:         100.0,
			SupportedModels: "",
			TotalCost:       42.56,
		},
		{
			Name:            "Development Key",
			APIKey:          "sk-wheel-demo-dev-bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			Enabled:         true,
			ExpireAt:        now.AddDate(0, 3, 0).Unix(),
			MaxCost:         10.0,
			SupportedModels: "gpt-4o-mini,claude-haiku-4-20250514",
			TotalCost:       3.21,
		},
		{
			Name:            "Testing Key",
			APIKey:          "sk-wheel-demo-test-cccccccccccccccccccccccccccccccccccccccc",
			Enabled:         true,
			ExpireAt:        now.AddDate(0, 0, 7).Unix(),
			MaxCost:         1.0,
			SupportedModels: "gpt-4o-mini",
			TotalCost:       0.87,
		},
		{
			Name:            "Expired Key",
			APIKey:          "sk-wheel-demo-exp-ddddddddddddddddddddddddddddddddddddddd",
			Enabled:         false,
			ExpireAt:        now.AddDate(0, 0, -30).Unix(),
			MaxCost:         50.0,
			SupportedModels: "",
			TotalCost:       49.99,
		},
	}

	for i := range keys {
		if _, err := db.NewInsert().Model(&keys[i]).Exec(ctx); err != nil {
			return err
		}
	}

	log.Printf("[seed] Created %d API keys", len(keys))
	return nil
}

func seedPricing(ctx context.Context, db *bun.DB) error {
	prices := []types.LLMPrice{
		{Name: "gpt-4o", InputPrice: 2.50, OutputPrice: 10.00, CacheReadPrice: 1.25, Source: "seed"},
		{Name: "gpt-4o-mini", InputPrice: 0.15, OutputPrice: 0.60, CacheReadPrice: 0.075, Source: "seed"},
		{Name: "gpt-4.1", InputPrice: 2.00, OutputPrice: 8.00, CacheReadPrice: 0.50, Source: "seed"},
		{Name: "gpt-4.1-mini", InputPrice: 0.40, OutputPrice: 1.60, CacheReadPrice: 0.10, Source: "seed"},
		{Name: "gpt-4.1-nano", InputPrice: 0.10, OutputPrice: 0.40, CacheReadPrice: 0.025, Source: "seed"},
		{Name: "o3-mini", InputPrice: 1.10, OutputPrice: 4.40, CacheReadPrice: 0.55, Source: "seed"},
		{Name: "claude-sonnet-4-20250514", InputPrice: 3.00, OutputPrice: 15.00, CacheReadPrice: 0.30, CacheWritePrice: 3.75, Source: "seed"},
		{Name: "claude-haiku-4-20250514", InputPrice: 0.80, OutputPrice: 4.00, CacheReadPrice: 0.08, CacheWritePrice: 1.00, Source: "seed"},
		{Name: "claude-3.5-sonnet-20241022", InputPrice: 3.00, OutputPrice: 15.00, CacheReadPrice: 0.30, CacheWritePrice: 3.75, Source: "seed"},
		{Name: "gemini-2.5-pro", InputPrice: 1.25, OutputPrice: 10.00, Source: "seed"},
		{Name: "gemini-2.5-flash", InputPrice: 0.15, OutputPrice: 0.60, Source: "seed"},
		{Name: "gemini-2.0-flash", InputPrice: 0.10, OutputPrice: 0.40, Source: "seed"},
	}

	for i := range prices {
		if _, err := db.NewInsert().Model(&prices[i]).Exec(ctx); err != nil {
			return err
		}
	}

	log.Printf("[seed] Created %d pricing entries", len(prices))
	return nil
}

func seedLogs(ctx context.Context, logDB *bun.DB) error {
	now := time.Now()
	rng := rand.New(rand.NewSource(42))

	type modelInfo struct {
		requestModel string
		actualModel  string
		channelID    int
		channelName  string
	}

	models := []modelInfo{
		{"gpt-4o", "gpt-4o-2024-11-20", 1, "OpenAI"},
		{"gpt-4o-mini", "gpt-4o-mini-2024-07-18", 1, "OpenAI"},
		{"gpt-4.1", "gpt-4.1-2025-04-14", 1, "OpenAI"},
		{"claude-sonnet-4-20250514", "claude-sonnet-4-20250514", 2, "Anthropic"},
		{"claude-haiku-4-20250514", "claude-haiku-4-20250514", 2, "Anthropic"},
		{"gemini-2.5-flash", "gemini-2.5-flash", 3, "Google Gemini"},
	}

	var logs []types.RelayLog
	// Generate logs for the past 30 days
	for day := 30; day >= 0; day-- {
		date := now.AddDate(0, 0, -day)
		// More requests on recent days
		numRequests := 5 + rng.Intn(25)
		if day < 7 {
			numRequests += 10
		}

		for i := 0; i < numRequests; i++ {
			m := models[rng.Intn(len(models))]
			hour := rng.Intn(16) + 7 // 7am–11pm
			minute := rng.Intn(60)
			ts := time.Date(date.Year(), date.Month(), date.Day(), hour, minute, rng.Intn(60), 0, time.Local)

			inputTokens := 50 + rng.Intn(2000)
			outputTokens := 20 + rng.Intn(1500)
			ftut := 200 + rng.Intn(2000)
			useTime := ftut + 500 + rng.Intn(5000)

			// Approximate cost based on pricing (per million tokens)
			var inputPrice, outputPrice float64
			switch m.requestModel {
			case "gpt-4o":
				inputPrice, outputPrice = 2.50, 10.00
			case "gpt-4o-mini":
				inputPrice, outputPrice = 0.15, 0.60
			case "gpt-4.1":
				inputPrice, outputPrice = 2.00, 8.00
			case "claude-sonnet-4-20250514":
				inputPrice, outputPrice = 3.00, 15.00
			case "claude-haiku-4-20250514":
				inputPrice, outputPrice = 0.80, 4.00
			case "gemini-2.5-flash":
				inputPrice, outputPrice = 0.15, 0.60
			}
			cost := float64(inputTokens)*inputPrice/1_000_000 + float64(outputTokens)*outputPrice/1_000_000

			errMsg := ""
			attemptStatus := types.AttemptStatusSuccess
			totalAttempts := 1

			// ~8% error rate
			if rng.Float64() < 0.08 {
				errMsg = randomError(rng)
				attemptStatus = types.AttemptStatusFailed
				totalAttempts = 1 + rng.Intn(3)
				cost = 0
			}

			attempts := types.AttemptList{
				{
					ChannelID:   m.channelID,
					ChannelName: m.channelName,
					ModelName:   m.actualModel,
					AttemptNum:  1,
					Status:      attemptStatus,
					Duration:    useTime,
				},
			}

			rl := types.RelayLog{
				Time:             ts.Unix(),
				RequestModelName: m.requestModel,
				ChannelID:        m.channelID,
				ChannelName:      m.channelName,
				ActualModelName:  m.actualModel,
				InputTokens:      inputTokens,
				OutputTokens:     outputTokens,
				FTUT:             ftut,
				UseTime:          useTime,
				Cost:             cost,
				RequestContent:   `{"model":"` + m.requestModel + `","messages":[{"role":"user","content":"Hello"}]}`,
				ResponseContent:  `{"id":"chatcmpl-demo","choices":[{"message":{"content":"Hi there!"}}]}`,
				Error:            errMsg,
				Attempts:         attempts,
				TotalAttempts:    totalAttempts,
			}
			logs = append(logs, rl)
		}
	}

	// Batch insert in chunks of 100
	for i := 0; i < len(logs); i += 100 {
		end := i + 100
		if end > len(logs) {
			end = len(logs)
		}
		batch := logs[i:end]
		if _, err := logDB.NewInsert().Model(&batch).Exec(ctx); err != nil {
			return err
		}
	}

	log.Printf("[seed] Created %d request logs", len(logs))
	return nil
}

func randomError(rng *rand.Rand) string {
	errors := []string{
		"upstream: 429 Too Many Requests",
		"upstream: 500 Internal Server Error",
		"upstream: 503 Service Unavailable",
		"context deadline exceeded (first token timeout)",
		"upstream: connection refused",
	}
	return errors[rng.Intn(len(errors))]
}

package service

import (
	"context"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// ImportData imports a DBDump into the database, deduplicating by name/key.
func ImportData(ctx context.Context, db *bun.DB, dump *types.DBDump) types.ImportResult {
	result := types.ImportResult{}

	// Import channels (dedup by name)
	if len(dump.Channels) > 0 {
		existingChannels, _ := dal.ListChannels(ctx, db)
		existingNames := make(map[string]bool)
		for _, ch := range existingChannels {
			existingNames[ch.Name] = true
		}

		for _, ch := range dump.Channels {
			if existingNames[ch.Name] {
				result.Channels.Skipped++
				continue
			}

			keys := make([]types.ChannelKeyInput, 0, len(ch.Keys))
			for _, k := range ch.Keys {
				keys = append(keys, types.ChannelKeyInput{
					ChannelKey: k.ChannelKey,
					Remark:     k.Remark,
				})
			}

			ch.ID = 0
			if _, err := dal.CreateChannel(ctx, db, ch, keys); err != nil {
				continue
			}
			result.Channels.Added++
		}
	}

	// Import groups (dedup by name)
	if len(dump.Groups) > 0 {
		existingGroups, _ := dal.ListGroups(ctx, db, 0)
		existingNames := make(map[string]bool)
		for _, g := range existingGroups {
			existingNames[g.Name] = true
		}

		for _, g := range dump.Groups {
			if existingNames[g.Name] {
				result.Groups.Skipped++
				continue
			}

			items := make([]types.GroupItemInput, 0, len(g.Items))
			for _, item := range g.Items {
				items = append(items, types.GroupItemInput{
					ChannelID: item.ChannelID,
					ModelName: item.ModelName,
					Priority:  item.Priority,
					Weight:    item.Weight,
				})
			}

			g.ID = 0
			if _, err := dal.CreateGroup(ctx, db, g, items); err != nil {
				continue
			}
			result.Groups.Added++
		}
	}

	// Import API keys (dedup by apiKey value)
	if len(dump.APIKeys) > 0 {
		existingKeys, _ := dal.ListApiKeys(ctx, db)
		existingKeyValues := make(map[string]bool)
		for _, k := range existingKeys {
			existingKeyValues[k.APIKey] = true
		}

		for _, ak := range dump.APIKeys {
			if existingKeyValues[ak.APIKey] {
				result.APIKeys.Skipped++
				continue
			}

			if _, err := dal.CreateApiKey(ctx, db, ak.Name, ak.ExpireAt, ak.MaxCost, ak.SupportedModels); err != nil {
				continue
			}
			result.APIKeys.Added++
		}
	}

	// Import settings (skip existing)
	if len(dump.Settings) > 0 {
		existingSettings, _ := dal.GetAllSettings(ctx, db)

		for _, s := range dump.Settings {
			if _, exists := existingSettings[s.Key]; exists {
				result.Settings.Skipped++
				continue
			}
			dal.UpdateSettings(ctx, db, map[string]string{s.Key: s.Value})
			result.Settings.Added++
		}
	}

	return result
}

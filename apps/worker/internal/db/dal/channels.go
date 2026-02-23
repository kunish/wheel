package dal

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListChannels(ctx context.Context, db *bun.DB) ([]types.Channel, error) {
	var channels []types.Channel
	err := db.NewSelect().Model(&channels).
		OrderExpr("`order` ASC, id ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Load all keys
	var allKeys []types.ChannelKey
	err = db.NewSelect().Model(&allKeys).Scan(ctx)
	if err != nil {
		return nil, err
	}

	keyMap := make(map[int][]types.ChannelKey)
	for _, k := range allKeys {
		keyMap[k.ChannelID] = append(keyMap[k.ChannelID], k)
	}

	for i := range channels {
		channels[i].Keys = keyMap[channels[i].ID]
		if channels[i].Keys == nil {
			channels[i].Keys = []types.ChannelKey{}
		}
	}

	return channels, nil
}

func GetChannel(ctx context.Context, db *bun.DB, id int) (*types.Channel, error) {
	ch := new(types.Channel)
	err := db.NewSelect().Model(ch).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	keys, err := listChannelKeys(ctx, db, id)
	if err != nil {
		return nil, err
	}
	ch.Keys = keys
	return ch, nil
}

func listChannelKeys(ctx context.Context, db *bun.DB, channelID int) ([]types.ChannelKey, error) {
	var keys []types.ChannelKey
	err := db.NewSelect().Model(&keys).Where("channel_id = ?", channelID).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []types.ChannelKey{}
	}
	return keys, nil
}

func CreateChannel(ctx context.Context, db *bun.DB, ch types.Channel, keys []types.ChannelKeyInput) (*types.Channel, error) {
	_, err := db.NewInsert().Model(&ch).Exec(ctx)
	if err != nil {
		return nil, err
	}

	if len(keys) > 0 {
		for _, k := range keys {
			ck := &types.ChannelKey{
				ChannelID:  ch.ID,
				ChannelKey: k.ChannelKey,
				Remark:     k.Remark,
				Enabled:    true,
			}
			_, err := db.NewInsert().Model(ck).Exec(ctx)
			if err != nil {
				return nil, err
			}
		}
	}

	return &ch, nil
}

var allowedChannelCols = map[string]bool{
	"name": true, "type": true, "enabled": true, "base_urls": true,
	"model": true, "fetched_model": true, "custom_model": true, "proxy": true, "auto_sync": true,
	"auto_group": true, "custom_header": true, "param_override": true,
	"channel_proxy": true,
}

func UpdateChannel(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	q := db.NewUpdate().Table("channels")
	for col, val := range data {
		if !allowedChannelCols[col] {
			continue
		}
		q = q.Set(col+" = ?", val)
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteChannel(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.Channel)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func EnableChannel(ctx context.Context, db *bun.DB, id int, enabled bool) error {
	_, err := db.NewUpdate().Table("channels").
		Set("enabled = ?", enabled).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func SyncChannelKeys(ctx context.Context, db *bun.DB, channelID int, keys []types.ChannelKeyInput) error {
	_, err := db.NewDelete().Model((*types.ChannelKey)(nil)).Where("channel_id = ?", channelID).Exec(ctx)
	if err != nil {
		return err
	}
	for _, k := range keys {
		ck := &types.ChannelKey{
			ChannelID:  channelID,
			ChannelKey: k.ChannelKey,
			Remark:     k.Remark,
			Enabled:    true,
		}
		if _, err := db.NewInsert().Model(ck).Exec(ctx); err != nil {
			return err
		}
	}
	return nil
}

func UpdateChannelKeyStatus(ctx context.Context, db *bun.DB, keyID int, statusCode int) error {
	now := time.Now().Unix()
	_, err := db.NewUpdate().Table("channel_keys").
		Set("status_code = ?", statusCode).
		Set("last_use_timestamp = ?", now).
		Where("id = ?", keyID).
		Exec(ctx)
	return err
}

func IncrementChannelKeyCost(ctx context.Context, db *bun.DB, keyID int, cost float64) error {
	_, err := db.NewUpdate().Table("channel_keys").
		Set("total_cost = total_cost + ?", cost).
		Where("id = ?", keyID).
		Exec(ctx)
	return err
}

// IncrementChannelKeyCostTx is the transaction-compatible variant of IncrementChannelKeyCost.
func IncrementChannelKeyCostTx(ctx context.Context, tx bun.Tx, keyID int, cost float64) error {
	_, err := tx.NewUpdate().Table("channel_keys").
		Set("total_cost = total_cost + ?", cost).
		Where("id = ?", keyID).
		Exec(ctx)
	return err
}

func ReorderChannels(ctx context.Context, db *bun.DB, orderedIDs []int) error {
	for i, id := range orderedIDs {
		_, err := db.NewUpdate().Table("channels").
			Set("`order` = ?", i).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

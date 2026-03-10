package dal

import (
	"context"
	"crypto/rand"
	"log"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"

func generateAPIKey() string {
	bytes := make([]byte, 48)
	_, _ = rand.Read(bytes)
	key := "sk-wheel-"
	for _, b := range bytes {
		key += string(charset[int(b)%len(charset)])
	}
	return key
}

func ListApiKeys(ctx context.Context, db *bun.DB) ([]types.APIKey, error) {
	var keys []types.APIKey
	err := db.NewSelect().Model(&keys).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []types.APIKey{}
	}
	return keys, nil
}

func CreateApiKey(ctx context.Context, db *bun.DB, name string, expireAt int64, maxCost float64, supportedModels string, rpmLimit int, tpmLimit int) (*types.APIKey, error) {
	key := generateAPIKey()
	ak := &types.APIKey{
		Name:            name,
		APIKey:          key,
		Enabled:         true,
		ExpireAt:        expireAt,
		MaxCost:         maxCost,
		SupportedModels: supportedModels,
		RPMLimit:        rpmLimit,
		TPMLimit:        tpmLimit,
	}
	_, err := db.NewInsert().Model(ak).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return ak, nil
}

var allowedApiKeyCols = map[string]bool{
	"name": true, "enabled": true, "expire_at": true,
	"max_cost": true, "supported_models": true,
	"rpm_limit": true, "tpm_limit": true,
}

func UpdateApiKey(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	q := db.NewUpdate().Table("api_keys")
	for col, val := range data {
		if !allowedApiKeyCols[col] {
			continue
		}
		q = q.Set(col+" = ?", val)
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteApiKey(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.APIKey)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func GetApiKeyByKey(ctx context.Context, db *bun.DB, key string) (*types.APIKey, error) {
	ak := new(types.APIKey)
	err := db.NewSelect().Model(ak).Where("api_key = ?", key).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return ak, nil
}

// EnsureDefaultApiKey creates a default API key if none exist yet.
// This runs at startup so new installations have a usable key immediately.
func EnsureDefaultApiKey(ctx context.Context, db *bun.DB) {
	count, err := db.NewSelect().Model((*types.APIKey)(nil)).Count(ctx)
	if err != nil {
		log.Printf("[startup] Failed to check API keys: %v", err)
		return
	}
	if count > 0 {
		return
	}

	key := generateAPIKey()
	ak := &types.APIKey{
		Name:    "Default",
		APIKey:  key,
		Enabled: true,
	}
	if _, err := db.NewInsert().Model(ak).Exec(ctx); err != nil {
		log.Printf("[startup] Failed to create default API key: %v", err)
		return
	}
	log.Printf("[startup] Created default API key: %s", key)
}

// IncrementApiKeyCostTx is the transaction-compatible variant of IncrementApiKeyCost.
func IncrementApiKeyCostTx(ctx context.Context, tx bun.Tx, id int, cost float64) error {
	_, err := tx.NewUpdate().Table("api_keys").
		Set("total_cost = total_cost + ?", cost).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

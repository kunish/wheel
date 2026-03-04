package dal

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func generateVirtualKey() string {
	bytes := make([]byte, 24)
	_, _ = rand.Read(bytes)
	return "vk-wheel-" + hex.EncodeToString(bytes)
}

func ListVirtualKeys(ctx context.Context, db *bun.DB) ([]types.VirtualKey, error) {
	var keys []types.VirtualKey
	err := db.NewSelect().Model(&keys).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if keys == nil {
		keys = []types.VirtualKey{}
	}
	return keys, nil
}

func CreateVirtualKey(ctx context.Context, db *bun.DB, req types.VirtualKeyCreateRequest) (*types.VirtualKey, error) {
	key := generateVirtualKey()

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			return nil, err
		}
		expiresAt = &t
	}

	vk := &types.VirtualKey{
		Name:          req.Name,
		Key:           key,
		Description:   req.Description,
		TeamID:        req.TeamID,
		ApiKeyID:      req.ApiKeyID,
		Enabled:       true,
		RateLimitRPM:  req.RateLimitRPM,
		RateLimitTPM:  req.RateLimitTPM,
		MaxBudget:     req.MaxBudget,
		AllowedModels: req.AllowedModels,
		ExpiresAt:     expiresAt,
	}

	_, err := db.NewInsert().Model(vk).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return vk, nil
}

var allowedVirtualKeyCols = map[string]bool{
	"name": true, "description": true, "enabled": true,
	"rate_limit_rpm": true, "rate_limit_tpm": true,
	"max_budget": true, "allowed_models": true,
	"updated_at": true,
}

func UpdateVirtualKey(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	data["updated_at"] = time.Now()

	q := db.NewUpdate().Table("virtual_keys")
	for col, val := range data {
		if !allowedVirtualKeyCols[col] {
			continue
		}
		q = q.Set(col+" = ?", val)
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteVirtualKey(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.VirtualKey)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func GetVirtualKeyByKey(ctx context.Context, db *bun.DB, key string) (*types.VirtualKey, error) {
	vk := new(types.VirtualKey)
	err := db.NewSelect().Model(vk).Where("key = ?", key).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return vk, nil
}

func IncrementVirtualKeySpend(ctx context.Context, db *bun.DB, id int, cost float64) error {
	_, err := db.NewUpdate().Table("virtual_keys").
		Set("current_spend = current_spend + ?", cost).
		Set("updated_at = ?", time.Now()).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

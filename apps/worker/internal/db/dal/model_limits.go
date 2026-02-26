package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ListModelLimits returns all model limits.
func ListModelLimits(ctx context.Context, db *bun.DB) ([]types.ModelLimit, error) {
	var limits []types.ModelLimit
	err := db.NewSelect().Model(&limits).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if limits == nil {
		limits = []types.ModelLimit{}
	}
	return limits, nil
}

// GetModelLimit returns a single model limit by ID.
func GetModelLimit(ctx context.Context, db *bun.DB, id int) (*types.ModelLimit, error) {
	limit := new(types.ModelLimit)
	err := db.NewSelect().Model(limit).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return limit, nil
}

// GetModelLimitByModel returns the limit config for a specific model name.
func GetModelLimitByModel(ctx context.Context, db *bun.DB, model string) (*types.ModelLimit, error) {
	limit := new(types.ModelLimit)
	err := db.NewSelect().Model(limit).Where("model = ? AND enabled = true", model).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return limit, nil
}

// CreateModelLimit inserts a new model limit.
func CreateModelLimit(ctx context.Context, db *bun.DB, limit *types.ModelLimit) error {
	_, err := db.NewInsert().Model(limit).Exec(ctx)
	return err
}

var allowedModelLimitCols = map[string]bool{
	"model": true, "rpm": true, "tpm": true,
	"daily_requests": true, "daily_tokens": true, "enabled": true,
}

// UpdateModelLimit updates a model limit by ID.
func UpdateModelLimit(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	q := db.NewUpdate().Table("model_limits")
	for col, val := range data {
		if !allowedModelLimitCols[col] {
			continue
		}
		q = q.Set(col+" = ?", val)
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

// DeleteModelLimit deletes a model limit by ID.
func DeleteModelLimit(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.ModelLimit)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

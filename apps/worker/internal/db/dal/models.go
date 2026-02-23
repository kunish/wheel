package dal

import (
	"context"
	"time"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListLLMPrices(ctx context.Context, db *bun.DB) ([]types.LLMPrice, error) {
	var prices []types.LLMPrice
	err := db.NewSelect().Model(&prices).OrderExpr("name").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if prices == nil {
		prices = []types.LLMPrice{}
	}
	return prices, nil
}

func GetLLMPriceByName(ctx context.Context, db *bun.DB, name string) (*types.LLMPrice, error) {
	p := new(types.LLMPrice)
	err := db.NewSelect().Model(p).Where("name = ?", name).Limit(1).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

func CreateLLMPrice(ctx context.Context, db *bun.DB, name string, inputPrice, outputPrice float64, source string) (*types.LLMPrice, error) {
	if source == "" {
		source = "manual"
	}
	p := &types.LLMPrice{
		Name:        name,
		InputPrice:  inputPrice,
		OutputPrice: outputPrice,
		Source:      source,
	}
	_, err := db.NewInsert().Model(p).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

var allowedLLMPriceCols = map[string]bool{
	"name": true, "input_price": true, "output_price": true,
	"cache_read_price": true, "cache_write_price": true, "source": true,
}

func UpdateLLMPrice(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	q := db.NewUpdate().Table("llm_prices")
	for col, val := range data {
		if col == "updated_at" {
			q = q.Set("updated_at = NOW()")
		} else if allowedLLMPriceCols[col] {
			q = q.Set(col+" = ?", val)
		}
	}
	// Always update updated_at
	q = q.Set("updated_at = NOW()")
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteLLMPrice(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.LLMPrice)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func UpsertLLMPrice(ctx context.Context, db *bun.DB, name string, inputPrice, outputPrice, cacheReadPrice, cacheWritePrice float64, source string) (string, error) {
	existing, err := GetLLMPriceByName(ctx, db, name)
	if err != nil {
		return "", err
	}
	if existing != nil {
		// Only update if source is 'sync'
		if existing.Source == "sync" {
			_, err = db.NewUpdate().Table("llm_prices").
				Set("input_price = ?", inputPrice).
				Set("output_price = ?", outputPrice).
				Set("cache_read_price = ?", cacheReadPrice).
				Set("cache_write_price = ?", cacheWritePrice).
				Set("updated_at = NOW()").
				Where("id = ?", existing.ID).
				Exec(ctx)
			if err != nil {
				return "", err
			}
		}
		return "updated", nil
	}
	p := &types.LLMPrice{
		Name:            name,
		InputPrice:      inputPrice,
		OutputPrice:     outputPrice,
		CacheReadPrice:  cacheReadPrice,
		CacheWritePrice: cacheWritePrice,
		Source:          source,
	}
	_, err = db.NewInsert().Model(p).Exec(ctx)
	if err != nil {
		return "", err
	}
	return "created", nil
}

func SetLastPriceSyncTime(ctx context.Context, db *bun.DB) error {
	now := currentTimestamp()
	s := &types.Setting{Key: "last_price_sync_time", Value: now}
	_, err := db.NewInsert().Model(s).
		On("DUPLICATE KEY UPDATE").
		Set("value = VALUES(value)").
		Exec(ctx)
	return err
}

func GetLastPriceSyncTime(ctx context.Context, db *bun.DB) (*string, error) {
	return GetSetting(ctx, db, "last_price_sync_time")
}

func currentTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}

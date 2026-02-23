package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func GetAllSettings(ctx context.Context, db *bun.DB) (map[string]string, error) {
	var settings []types.Setting
	err := db.NewSelect().Model(&settings).Scan(ctx)
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)
	for _, s := range settings {
		result[s.Key] = s.Value
	}
	return result, nil
}

func GetSetting(ctx context.Context, db *bun.DB, key string) (*string, error) {
	s := new(types.Setting)
	err := db.NewSelect().Model(s).Where("key = ?", key).Limit(1).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return &s.Value, nil
}

func UpdateSettings(ctx context.Context, db *bun.DB, data map[string]string) error {
	for k, v := range data {
		s := &types.Setting{Key: k, Value: v}
		_, err := db.NewInsert().Model(s).
			On("DUPLICATE KEY UPDATE").
			Set("value = VALUES(value)").
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

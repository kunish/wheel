package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListTags(ctx context.Context, db *bun.DB) ([]types.Tag, error) {
	var tags []types.Tag
	err := db.NewSelect().Model(&tags).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if tags == nil {
		tags = []types.Tag{}
	}
	return tags, nil
}

func CreateTag(ctx context.Context, db *bun.DB, tag *types.Tag) error {
	_, err := db.NewInsert().Model(tag).Exec(ctx)
	return err
}

func UpdateTag(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	allowed := map[string]bool{
		"name": true, "color": true, "description": true,
	}
	q := db.NewUpdate().Table("tags")
	count := 0
	for col, val := range data {
		if allowed[col] {
			q = q.Set(col+" = ?", val)
			count++
		}
	}
	if count == 0 {
		return nil // nothing to update
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteTag(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.Tag)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

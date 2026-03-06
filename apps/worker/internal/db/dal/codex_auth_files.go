package dal

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

func ListCodexAuthFiles(ctx context.Context, db *bun.DB, channelID int) ([]types.CodexAuthFile, error) {
	var items []types.CodexAuthFile
	err := db.NewSelect().
		Model(&items).
		Where("channel_id = ?", channelID).
		OrderExpr("name ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []types.CodexAuthFile{}
	}
	return items, nil
}

func GetCodexAuthFileByName(ctx context.Context, db *bun.DB, channelID int, name string) (*types.CodexAuthFile, error) {
	item := new(types.CodexAuthFile)
	err := db.NewSelect().
		Model(item).
		Where("channel_id = ?", channelID).
		Where("name = ?", name).
		Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) || err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return item, nil
}

func CreateCodexAuthFile(ctx context.Context, db *bun.DB, item *types.CodexAuthFile) error {
	_, err := db.NewInsert().Model(item).Exec(ctx)
	return err
}

func UpdateCodexAuthFile(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	if len(data) == 0 {
		return nil
	}
	q := db.NewUpdate().Table("codex_auth_files")
	for col, val := range data {
		switch col {
		case "name", "provider", "email", "disabled", "content", "updated_at":
			q = q.Set(col+" = ?", val)
		}
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteCodexAuthFile(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.CodexAuthFile)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func DeleteAllCodexAuthFilesByChannel(ctx context.Context, db *bun.DB, channelID int) error {
	_, err := db.NewDelete().Model((*types.CodexAuthFile)(nil)).Where("channel_id = ?", channelID).Exec(ctx)
	return err
}

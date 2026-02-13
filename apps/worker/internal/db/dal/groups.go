package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListGroups(ctx context.Context, db *bun.DB) ([]types.Group, error) {
	var groups []types.Group
	err := db.NewSelect().Model(&groups).
		OrderExpr("\"order\" ASC, id ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}

	// Load all group items
	var allItems []types.GroupItem
	err = db.NewSelect().Model(&allItems).Scan(ctx)
	if err != nil {
		return nil, err
	}

	itemMap := make(map[int][]types.GroupItem)
	for _, item := range allItems {
		itemMap[item.GroupID] = append(itemMap[item.GroupID], item)
	}

	for i := range groups {
		groups[i].Items = itemMap[groups[i].ID]
		if groups[i].Items == nil {
			groups[i].Items = []types.GroupItem{}
		}
	}

	return groups, nil
}

func GetGroup(ctx context.Context, db *bun.DB, id int) (*types.Group, error) {
	g := new(types.Group)
	err := db.NewSelect().Model(g).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}

	items, err := listGroupItems(ctx, db, id)
	if err != nil {
		return nil, err
	}
	g.Items = items
	return g, nil
}

func listGroupItems(ctx context.Context, db *bun.DB, groupID int) ([]types.GroupItem, error) {
	var items []types.GroupItem
	err := db.NewSelect().Model(&items).Where("group_id = ?", groupID).Scan(ctx)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []types.GroupItem{}
	}
	return items, nil
}

func CreateGroup(ctx context.Context, db *bun.DB, g types.Group, items []types.GroupItemInput) (*types.Group, error) {
	_, err := db.NewInsert().Model(&g).Exec(ctx)
	if err != nil {
		return nil, err
	}

	if len(items) > 0 {
		for _, item := range items {
			enabled := true
			if item.Enabled != nil {
				enabled = *item.Enabled
			}
			gi := &types.GroupItem{
				GroupID:   g.ID,
				ChannelID: item.ChannelID,
				ModelName: item.ModelName,
				Priority:  item.Priority,
				Weight:    item.Weight,
				Enabled:   enabled,
			}
			if _, err := db.NewInsert().Model(gi).Exec(ctx); err != nil {
				return nil, err
			}
		}
	}

	return &g, nil
}

func UpdateGroup(ctx context.Context, db *bun.DB, id int, data map[string]any, items []types.GroupItemInput, replaceItems bool) error {
	allowedGroupCols := map[string]bool{
		"name": true, "mode": true, "enabled": true, "order": true,
		"first_token_time_out": true, "session_keep_time": true,
	}
	if len(data) > 0 {
		q := db.NewUpdate().Table("groups")
		for col, val := range data {
			if col == "order" || col == `"order"` {
				q = q.Set("\"order\" = ?", val)
			} else if allowedGroupCols[col] {
				q = q.Set(col+" = ?", val)
			}
		}
		if _, err := q.Where("id = ?", id).Exec(ctx); err != nil {
			return err
		}
	}

	if replaceItems {
		if _, err := db.NewDelete().Model((*types.GroupItem)(nil)).Where("group_id = ?", id).Exec(ctx); err != nil {
			return err
		}
		for _, item := range items {
			enabled := true
			if item.Enabled != nil {
				enabled = *item.Enabled
			}
			gi := &types.GroupItem{
				GroupID:   id,
				ChannelID: item.ChannelID,
				ModelName: item.ModelName,
				Priority:  item.Priority,
				Weight:    item.Weight,
				Enabled:   enabled,
			}
			if _, err := db.NewInsert().Model(gi).Exec(ctx); err != nil {
				return err
			}
		}
	}

	return nil
}

func DeleteGroup(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.Group)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func ReorderGroups(ctx context.Context, db *bun.DB, orderedIDs []int) error {
	for i, id := range orderedIDs {
		_, err := db.NewUpdate().Table("groups").
			Set("\"order\" = ?", i).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

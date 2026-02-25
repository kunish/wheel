package dal

import (
	"context"
	"slices"
	"strings"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListGroups(ctx context.Context, db *bun.DB, profileID int) ([]types.Group, error) {
	var groups []types.Group
	q := db.NewSelect().Model(&groups)
	if profileID > 0 {
		q = q.Where("profile_id = ?", profileID)
	}
	err := q.OrderExpr("`order` ASC, id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []types.Group{}
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
				q = q.Set("`order` = ?", val)
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
	if _, err := db.NewDelete().Model((*types.GroupItem)(nil)).Where("group_id = ?", id).Exec(ctx); err != nil {
		return err
	}
	_, err := db.NewDelete().Model((*types.Group)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

func ReorderGroups(ctx context.Context, db *bun.DB, orderedIDs []int) error {
	for i, id := range orderedIDs {
		_, err := db.NewUpdate().Table("groups").
			Set("`order` = ?", i).
			Where("id = ?", id).
			Exec(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

// AssignOrphanedGroups migrates groups with profile_id=0 to defaultProfileID.
func AssignOrphanedGroups(ctx context.Context, db *bun.DB, defaultProfileID int) error {
	_, err := db.NewUpdate().TableExpr("`groups`").
		Set("profile_id = ?", defaultProfileID).
		Where("profile_id = 0").
		Exec(ctx)
	return err
}

func ListGroupsByProfileLite(ctx context.Context, db *bun.DB, profileID int) ([]types.Group, error) {
	var groups []types.Group
	err := db.NewSelect().Model(&groups).
		Where("profile_id = ?", profileID).
		OrderExpr("`order` ASC, id ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if groups == nil {
		groups = []types.Group{}
	}
	return groups, nil
}

func MaterializeProfileGroups(
	ctx context.Context,
	db *bun.DB,
	profileID int,
	modelNames []string,
) (created int, existing int, err error) {
	if profileID <= 0 {
		return 0, 0, nil
	}

	seen := make(map[string]struct{}, len(modelNames))
	uniqueNames := make([]string, 0, len(modelNames))
	for _, modelName := range modelNames {
		name := strings.TrimSpace(modelName)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		uniqueNames = append(uniqueNames, name)
	}
	if len(uniqueNames) == 0 {
		return 0, 0, nil
	}
	slices.Sort(uniqueNames)

	var existedGroups []types.Group
	if err = db.NewSelect().Model(&existedGroups).
		Where("profile_id = ?", profileID).
		Where("name IN (?)", bun.List(uniqueNames)).
		Scan(ctx); err != nil {
		return 0, 0, err
	}
	existingNameSet := make(map[string]struct{}, len(existedGroups))
	for _, g := range existedGroups {
		existingNameSet[g.Name] = struct{}{}
	}
	existing = len(existedGroups)

	for _, name := range uniqueNames {
		if _, ok := existingNameSet[name]; ok {
			continue
		}
		_, err = CreateGroup(ctx, db, types.Group{
			Name:      name,
			Mode:      types.GroupModeRoundRobin,
			ProfileID: profileID,
		}, nil)
		if err != nil {
			return created, existing, err
		}
		created++
	}
	return created, existing, nil
}

// ResetGroupProfilesOnce resets all groups' profile_id to 0 exactly once,
// using a settings key as guard. This fixes data corrupted by earlier buggy assignment logic.
func ResetGroupProfilesOnce(ctx context.Context, db *bun.DB) {
	var count int
	count, _ = db.NewSelect().TableExpr("settings").
		Where("`key` = ?", "profile_id_reset_v2").
		Count(ctx)
	if count > 0 {
		return
	}
	_, _ = db.NewUpdate().TableExpr("`groups`").
		Set("profile_id = 0").
		Where("profile_id != 0").
		Exec(ctx)
	_, _ = db.ExecContext(ctx, "INSERT INTO settings (`key`, value) VALUES (?, ?)", "profile_id_reset_v2", "done")
}

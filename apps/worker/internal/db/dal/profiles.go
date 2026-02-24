package dal

import (
	"context"
	"database/sql"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListProfiles(ctx context.Context, db *bun.DB) ([]types.ModelProfile, error) {
	var profiles []types.ModelProfile
	err := db.NewSelect().Model(&profiles).
		OrderExpr("is_builtin DESC, CASE WHEN name = 'Default' THEN 0 ELSE 1 END, name ASC").
		Scan(ctx)
	if err != nil {
		return nil, err
	}
	if profiles == nil {
		profiles = []types.ModelProfile{}
	}

	// Populate group counts per profile
	if len(profiles) > 0 {
		var counts []struct {
			ProfileID  int `bun:"profile_id"`
			GroupCount int `bun:"group_count"`
		}
		err = db.NewSelect().TableExpr("`groups`").
			ColumnExpr("profile_id").
			ColumnExpr("COUNT(*) AS group_count").
			Group("profile_id").
			Scan(ctx, &counts)
		if err == nil {
			countMap := make(map[int]int, len(counts))
			for _, c := range counts {
				countMap[c.ProfileID] = c.GroupCount
			}
			for i := range profiles {
				profiles[i].GroupCount = countMap[profiles[i].ID]
			}
		}
	}

	return profiles, nil
}

func GetProfile(ctx context.Context, db *bun.DB, id int) (*types.ModelProfile, error) {
	p := new(types.ModelProfile)
	err := db.NewSelect().Model(p).Where("id = ?", id).Scan(ctx)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return p, nil
}

// NormalizeModels deduplicates, trims, and sorts model names.
func NormalizeModels(models []string) []string {
	if len(models) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(models))
	result := make([]string, 0, len(models))
	for _, model := range models {
		m := strings.TrimSpace(model)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		result = append(result, m)
	}
	sort.Strings(result)
	return result
}

func CreateProfile(
	ctx context.Context,
	db *bun.DB,
	name string,
	provider string,
	models []string,
) (*types.ModelProfile, error) {
	p := &types.ModelProfile{
		Name:      name,
		Provider:  strings.TrimSpace(provider),
		Models:    types.StringList(NormalizeModels(models)),
		IsBuiltin: false,
	}
	if p.Provider == "" {
		p.Provider = "custom"
	}
	_, err := db.NewInsert().Model(p).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func UpdateProfile(
	ctx context.Context,
	db *bun.DB,
	id int,
	name string,
	provider *string,
	models []string,
	replaceModels bool,
) error {
	q := db.NewUpdate().Table("model_profiles").
		Set("name = ?", name).
		Set("updated_at = NOW()").
		Where("id = ?", id).
		Where("is_builtin = false")
	if provider != nil {
		q = q.Set("provider = ?", strings.TrimSpace(*provider))
	}
	if replaceModels {
		q = q.Set("models = ?", types.StringList(NormalizeModels(models)))
	}
	res, err := q.Exec(ctx)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("profile not found or is builtin")
	}
	return nil
}

func DeleteProfile(ctx context.Context, db *bun.DB, id int) error {
	res, err := db.NewDelete().Model((*types.ModelProfile)(nil)).
		Where("id = ?", id).
		Where("is_builtin = false").
		Exec(ctx)
	if err != nil {
		return err
	}
	rows, _ := res.RowsAffected()
	if rows == 0 {
		return errors.New("profile not found or is builtin")
	}
	return nil
}

func UpsertBuiltinProfile(
	ctx context.Context,
	db *bun.DB,
	name string,
	provider string,
	models []string,
) error {
	existing := new(types.ModelProfile)
	err := db.NewSelect().Model(existing).
		Where("name = ?", name).
		Where("is_builtin = true").
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	normalizedProvider := strings.TrimSpace(provider)
	if normalizedProvider == "" {
		normalizedProvider = "custom"
	}
	shouldUpdateModels := models != nil
	normalizedModels := types.StringList(NormalizeModels(models))
	if len(normalizedModels) > 1 {
		slices.Sort(normalizedModels)
	}

	if existing.ID > 0 && err == nil {
		q := db.NewUpdate().Table("model_profiles").
			Set("provider = ?", normalizedProvider).
			Set("updated_at = NOW()").
			Where("id = ?", existing.ID)
		if shouldUpdateModels {
			q = q.Set("models = ?", normalizedModels)
		}
		_, err = q.Exec(ctx)
		return err
	}
	p := &types.ModelProfile{
		Name:      name,
		Provider:  normalizedProvider,
		Models:    normalizedModels,
		IsBuiltin: true,
	}
	_, err = db.NewInsert().Model(p).Exec(ctx)
	return err
}

// EnsureDefaultProfile ensures a "Default" builtin profile exists and returns its ID.
func EnsureDefaultProfile(ctx context.Context, db *bun.DB) (int, error) {
	p := new(types.ModelProfile)
	err := db.NewSelect().Model(p).
		Where("name = ?", "Default").
		Where("is_builtin = true").
		Scan(ctx)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return 0, err
	}
	if p.ID > 0 {
		return p.ID, nil
	}
	np := &types.ModelProfile{
		Name:      "Default",
		Provider:  "system",
		Models:    types.StringList{},
		IsBuiltin: true,
	}
	_, err = db.NewInsert().Model(np).Exec(ctx)
	if err != nil {
		return 0, err
	}
	return np.ID, nil
}

// CountGroupsByProfile returns the number of groups belonging to a profile.
func CountGroupsByProfile(ctx context.Context, db *bun.DB, profileID int) (int, error) {
	count, err := db.NewSelect().TableExpr("`groups`").
		Where("profile_id = ?", profileID).
		Count(ctx)
	return count, err
}

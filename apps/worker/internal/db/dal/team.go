package dal

import (
	"context"

	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// ListTeams returns all teams.
func ListTeams(ctx context.Context, db *bun.DB) ([]types.Team, error) {
	var teams []types.Team
	err := db.NewSelect().Model(&teams).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if teams == nil {
		teams = []types.Team{}
	}
	return teams, nil
}

// GetTeam returns a single team by ID.
func GetTeam(ctx context.Context, db *bun.DB, id int) (*types.Team, error) {
	var team types.Team
	err := db.NewSelect().Model(&team).Where("id = ?", id).Scan(ctx)
	if err != nil {
		return nil, err
	}
	return &team, nil
}

// CreateTeam inserts a new team.
func CreateTeam(ctx context.Context, db *bun.DB, team *types.Team) error {
	_, err := db.NewInsert().Model(team).Exec(ctx)
	return err
}

// UpdateTeam updates a team.
func UpdateTeam(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	allowed := map[string]bool{"name": true, "description": true, "max_budget": true}
	q := db.NewUpdate().Table("teams")
	count := 0
	for col, val := range data {
		if allowed[col] {
			q = q.Set(col+" = ?", val)
			count++
		}
	}
	if count == 0 {
		return nil
	}
	_, err := q.Where("id = ?", id).Exec(ctx)
	return err
}

// DeleteTeam deletes a team by ID.
func DeleteTeam(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.Team)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

// GetTeamBudgetSummaries returns aggregated budget info for all teams.
func GetTeamBudgetSummaries(ctx context.Context, db *bun.DB) ([]types.TeamBudgetSummary, error) {
	teams, err := ListTeams(ctx, db)
	if err != nil {
		return nil, err
	}

	var summaries []types.TeamBudgetSummary
	for _, t := range teams {
		var vks []types.VirtualKey
		err := db.NewSelect().Model(&vks).Where("team_id = ?", t.ID).Scan(ctx)
		if err != nil {
			continue
		}

		totalSpend := 0.0
		for _, vk := range vks {
			totalSpend += vk.CurrentSpend
		}

		budgetPc := 0.0
		if t.MaxBudget > 0 {
			budgetPc = (totalSpend / t.MaxBudget) * 100
		}

		summaries = append(summaries, types.TeamBudgetSummary{
			TeamID:       t.ID,
			TeamName:     t.Name,
			MaxBudget:    t.MaxBudget,
			TotalSpend:   totalSpend,
			VirtualKeys:  len(vks),
			BudgetUsedPc: budgetPc,
		})
	}

	if summaries == nil {
		summaries = []types.TeamBudgetSummary{}
	}
	return summaries, nil
}

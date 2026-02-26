package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListRoutingRules(ctx context.Context, db *bun.DB) ([]types.RoutingRuleModel, error) {
	var rules []types.RoutingRuleModel
	err := db.NewSelect().Model(&rules).OrderExpr("priority ASC, id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []types.RoutingRuleModel{}
	}
	return rules, nil
}

func CreateRoutingRule(ctx context.Context, db *bun.DB, rule *types.RoutingRuleModel) error {
	_, err := db.NewInsert().Model(rule).Exec(ctx)
	return err
}

func UpdateRoutingRule(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	allowed := map[string]bool{
		"name": true, "priority": true, "enabled": true,
		"conditions": true, "action": true,
	}
	q := db.NewUpdate().Table("routing_rules")
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

func DeleteRoutingRule(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.RoutingRuleModel)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

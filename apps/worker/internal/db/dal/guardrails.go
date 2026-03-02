package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func ListGuardrailRules(ctx context.Context, db *bun.DB) ([]types.GuardrailRule, error) {
	var rules []types.GuardrailRule
	err := db.NewSelect().Model(&rules).OrderExpr("id ASC").Scan(ctx)
	if err != nil {
		return nil, err
	}
	if rules == nil {
		rules = []types.GuardrailRule{}
	}
	return rules, nil
}

func CreateGuardrailRule(ctx context.Context, db *bun.DB, rule *types.GuardrailRule) error {
	_, err := db.NewInsert().Model(rule).Exec(ctx)
	return err
}

func UpdateGuardrailRule(ctx context.Context, db *bun.DB, id int, data map[string]any) error {
	allowed := map[string]bool{
		"name": true, "type": true, "target": true,
		"action": true, "pattern": true, "max_length": true,
		"enabled": true,
	}
	q := db.NewUpdate().Table("guardrail_rules")
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

func DeleteGuardrailRule(ctx context.Context, db *bun.DB, id int) error {
	_, err := db.NewDelete().Model((*types.GuardrailRule)(nil)).Where("id = ?", id).Exec(ctx)
	return err
}

package dal

import (
	"context"

	"github.com/uptrace/bun"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

func GetUser(ctx context.Context, db *bun.DB) (*types.User, error) {
	u := new(types.User)
	err := db.NewSelect().Model(u).Limit(1).Scan(ctx)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, err
	}
	return u, nil
}

func CreateUser(ctx context.Context, db *bun.DB, username, hashedPassword string) (*types.User, error) {
	u := &types.User{Username: username, Password: hashedPassword}
	_, err := db.NewInsert().Model(u).Exec(ctx)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func UpdatePassword(ctx context.Context, db *bun.DB, id int, hashedPassword string) error {
	_, err := db.NewUpdate().Model((*types.User)(nil)).
		Set("password = ?", hashedPassword).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

func UpdateUsername(ctx context.Context, db *bun.DB, id int, username string) error {
	_, err := db.NewUpdate().Model((*types.User)(nil)).
		Set("username = ?", username).
		Where("id = ?", id).
		Exec(ctx)
	return err
}

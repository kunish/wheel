package bifrostx

import (
	"context"

	bifrost "github.com/maximhq/bifrost/core"
	schemas "github.com/maximhq/bifrost/core/schemas"
	"github.com/uptrace/bun"
)

type Client struct {
	core *bifrost.Bifrost
}

func New(ctx context.Context, db *bun.DB, sendBackRaw bool) (*Client, error) {
	account := NewAccount(db, sendBackRaw)
	core, err := bifrost.Init(ctx, schemas.BifrostConfig{
		Account:         account,
		InitialPoolSize: 128,
	})
	if err != nil {
		return nil, err
	}
	return &Client{core: core}, nil
}

func (c *Client) Core() *bifrost.Bifrost {
	if c == nil {
		return nil
	}
	return c.core
}

func (c *Client) EnsureProvider(channelID int) error {
	if c == nil || c.core == nil {
		return nil
	}
	return c.core.UpdateProvider(ProviderKeyForChannelID(channelID))
}

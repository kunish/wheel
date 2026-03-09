package handler

import (
	"context"
	"errors"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestExecuteFeatureWithRetry_NonProxyErrorStopsImmediately(t *testing.T) {
	model := "test-model"
	h := newTestRelayHandler(t, model,
		[]types.Channel{
			makeChannel(1, "https://example.com", 601, "k1"),
			makeChannel(2, "https://example.com", 602, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: model, Priority: 2, Enabled: true},
		},
	)

	calls := 0
	execErr := errors.New("payload build failed")
	err := h.executeFeatureWithRetry(context.Background(), model, 1, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		calls++
		return execErr
	})

	if !errors.Is(err, execErr) {
		t.Fatalf("expected original error, got %v", err)
	}
	if calls != 1 {
		t.Fatalf("expected immediate stop after non-proxy error, calls=%d", calls)
	}
}

func TestExecuteFeatureWithRetry_ProxyErrorRetries(t *testing.T) {
	model := "test-model"
	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, "https://example.com", 701, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	calls := 0
	err := h.executeFeatureWithRetry(context.Background(), model, 1, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		calls++
		return &relay.ProxyError{Message: "upstream failed", StatusCode: 500}
	})

	if err == nil {
		t.Fatal("expected proxy error to bubble up after retries")
	}
	if calls != maxRetryRounds {
		t.Fatalf("expected %d retries for proxy errors, got %d", maxRetryRounds, calls)
	}
}

func TestExecuteFeatureWithRetry_Proxy429WithNilDBDoesNotPanic(t *testing.T) {
	model := "test-model"
	h := newTestRelayHandler(t, model,
		[]types.Channel{makeChannel(1, "https://example.com", 801, "k1")},
		[]types.GroupItem{{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true}},
	)

	h.DB = nil

	calls := 0
	err := h.executeFeatureWithRetry(context.Background(), model, 1, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		calls++
		return &relay.ProxyError{Message: "rate limited", StatusCode: 429}
	})

	if err == nil {
		t.Fatal("expected proxy error after retries")
	}
	if calls != maxRetryRounds {
		t.Fatalf("expected %d attempts, got %d", maxRetryRounds, calls)
	}
}

func TestExecuteFeatureWithRetry_Proxy4xxStopsImmediately(t *testing.T) {
	model := "test-model"
	h := newTestRelayHandler(t, model,
		[]types.Channel{
			makeChannel(1, "https://example.com", 901, "k1"),
			makeChannel(2, "https://example.com", 902, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: model, Priority: 2, Enabled: true},
		},
	)

	calls := 0
	err := h.executeFeatureWithRetry(context.Background(), model, 1, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		calls++
		return &relay.ProxyError{Message: "unauthorized", StatusCode: 401}
	})

	if err == nil {
		t.Fatal("expected proxy error to bubble up")
	}
	if calls != 1 {
		t.Fatalf("expected non-retryable 4xx to stop immediately, calls=%d", calls)
	}
}

func TestExecuteFeatureWithRetry_ClientDisconnectStopsImmediately(t *testing.T) {
	model := "test-model"
	h := newTestRelayHandler(t, model,
		[]types.Channel{
			makeChannel(1, "https://example.com", 911, "k1"),
			makeChannel(2, "https://example.com", 912, "k2"),
		},
		[]types.GroupItem{
			{GroupID: 1, ChannelID: 1, ModelName: model, Priority: 1, Enabled: true},
			{GroupID: 1, ChannelID: 2, ModelName: model, Priority: 2, Enabled: true},
		},
	)

	calls := 0
	err := h.executeFeatureWithRetry(context.Background(), model, 1, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		calls++
		return &relay.ProxyError{Message: "client disconnected", StatusCode: 499}
	})

	if err == nil {
		t.Fatal("expected proxy error to bubble up")
	}
	if calls != 1 {
		t.Fatalf("expected client disconnect error to stop immediately, calls=%d", calls)
	}
}

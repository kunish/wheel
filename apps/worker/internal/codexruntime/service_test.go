package codexruntime

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

func TestNewFromConfigManagedDefaults(t *testing.T) {
	cfg := &config.Config{}

	svc, err := NewFromConfig(cfg)
	if err != nil {
		t.Fatalf("expected nil error with managed defaults, got %v", err)
	}
	if svc == nil {
		t.Fatalf("expected non-nil service with managed defaults")
	}
}

func TestServiceStartPropagatesRunError(t *testing.T) {
	expectedErr := errors.New("run failed")
	svc := &Service{run: func(context.Context) error { return expectedErr }}

	errCh := svc.Start(context.Background())
	err := <-errCh
	if !errors.Is(err, expectedErr) {
		t.Fatalf("expected %v, got %v", expectedErr, err)
	}
}

func TestManagedAuthFilePathFlattensChannelName(t *testing.T) {
	path := ManagedAuthFilePath(&types.CodexAuthFile{
		ChannelID: 7,
		Name:      "first.json",
	})

	if got, want := filepath.Dir(path), ManagedAuthDir(); got != want {
		t.Fatalf("managed auth dir = %q, want %q", got, want)
	}
	if got, want := filepath.Base(path), "channel-7--first.json"; got != want {
		t.Fatalf("managed auth file name = %q, want %q", got, want)
	}
}

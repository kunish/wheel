package main

import (
	"context"
	"errors"
	"log"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/config"
)

type fakeCodexRuntimeService struct {
	errCh <-chan error
}

func (f *fakeCodexRuntimeService) Start(context.Context) <-chan error {
	return f.errCh
}

func TestInitEmbeddedCodexRuntimeNilConfigSkips(t *testing.T) {
	var cfg *config.Config
	called := false

	errCh, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		called = true
		return nil, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if errCh != nil {
		t.Fatalf("expected nil err channel when config is nil")
	}
	if called {
		t.Fatalf("factory should not be called when config is nil")
	}
}

func TestInitEmbeddedCodexRuntimeAlwaysFailFast(t *testing.T) {
	cfg := &config.Config{}
	factoryErr := errors.New("startup failed")

	errCh, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		return nil, factoryErr
	})

	if !errors.Is(err, factoryErr) {
		t.Fatalf("expected %v, got %v", factoryErr, err)
	}
	if errCh != nil {
		t.Fatalf("expected nil err channel on startup error")
	}
}

func TestInitEmbeddedCodexRuntimeStartsService(t *testing.T) {
	cfg := &config.Config{}
	ch := make(chan error, 1)
	ch <- nil
	close(ch)

	errCh, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		return &fakeCodexRuntimeService{errCh: ch}, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if errCh == nil {
		t.Fatalf("expected non-nil err channel when service starts")
	}
}

func TestInitEmbeddedCodexRuntimeAutoGeneratesManagementKey(t *testing.T) {
	cfg := &config.Config{}
	ch := make(chan error, 1)
	ch <- nil
	close(ch)

	_, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(got *config.Config) (codexRuntimeService, error) {
		if got.CodexRuntimeManagementKey == "" {
			t.Fatal("expected management key to be auto-generated")
		}
		return &fakeCodexRuntimeService{errCh: ch}, nil
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

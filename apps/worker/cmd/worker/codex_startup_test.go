package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/config"
)

type fakeCodexRuntimeService struct {
	errCh   <-chan error
	handler http.Handler
}

func (f *fakeCodexRuntimeService) Start(context.Context) <-chan error {
	return f.errCh
}

func (f *fakeCodexRuntimeService) Handler() http.Handler {
	if f.handler != nil {
		return f.handler
	}
	return http.NewServeMux()
}

func TestInitEmbeddedCodexRuntimeNilConfigSkips(t *testing.T) {
	var cfg *config.Config
	called := false

	result, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		called = true
		return nil, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result != nil {
		t.Fatalf("expected nil result when config is nil")
	}
	if called {
		t.Fatalf("factory should not be called when config is nil")
	}
}

func TestInitEmbeddedCodexRuntimeAlwaysFailFast(t *testing.T) {
	cfg := &config.Config{}
	factoryErr := errors.New("startup failed")

	result, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		return nil, factoryErr
	})

	if !errors.Is(err, factoryErr) {
		t.Fatalf("expected %v, got %v", factoryErr, err)
	}
	if result != nil {
		t.Fatalf("expected nil result on startup error")
	}
}

func TestInitEmbeddedCodexRuntimeStartsService(t *testing.T) {
	cfg := &config.Config{}
	ch := make(chan error, 1)
	ch <- nil
	close(ch)

	result, err := initEmbeddedCodexRuntime(context.Background(), cfg, log.Default(), func(*config.Config) (codexRuntimeService, error) {
		return &fakeCodexRuntimeService{errCh: ch}, nil
	})

	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result when service starts")
	}
	if result.errCh == nil {
		t.Fatalf("expected non-nil errCh when service starts")
	}
	if result.handler == nil {
		t.Fatalf("expected non-nil handler when service starts")
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

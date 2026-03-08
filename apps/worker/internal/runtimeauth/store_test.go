package runtimeauth

import (
	"context"
	"path/filepath"
	"testing"

	cliproxyauth "github.com/kunish/wheel/apps/worker/third_party/CLIProxyAPIPlus/sdk/cliproxy/auth"
)

func TestRegisterTokenStoreSaveAndListAuth(t *testing.T) {
	t.Parallel()

	authDir := t.TempDir()
	store := NewFileTokenStore()
	store.SetBaseDir(authDir)
	RegisterTokenStore(store)
	t.Cleanup(func() { RegisterTokenStore(nil) })

	record := &cliproxyauth.Auth{
		ID:       "github-copilot.json",
		Provider: "github-copilot",
		FileName: "github-copilot.json",
		Metadata: map[string]any{
			"type":         "github-copilot",
			"email":        "copilot@example.com",
			"access_token": "ghu_test",
		},
	}

	savedPath, err := GetTokenStore().Save(context.Background(), record)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	if got, want := savedPath, filepath.Join(authDir, "github-copilot.json"); got != want {
		t.Fatalf("saved path = %q, want %q", got, want)
	}

	loaded, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(loaded) != 1 {
		t.Fatalf("List() len = %d, want 1", len(loaded))
	}
	if got := loaded[0].Provider; got != "github-copilot" {
		t.Fatalf("loaded provider = %q, want github-copilot", got)
	}
	if got := loaded[0].Label; got != "copilot@example.com" {
		t.Fatalf("loaded label = %q, want copilot@example.com", got)
	}
}

func TestNewLegacyAuthManagerUsesOwnedRegisteredStore(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	RegisterTokenStore(store)
	t.Cleanup(func() { RegisterTokenStore(nil) })

	mgr := NewLegacyAuthManager()
	if mgr == nil {
		t.Fatal("expected manager")
	}
	_, err := mgr.SaveAuth(&cliproxyauth.Auth{ID: "auth-id", FileName: "auth.json"}, nil)
	if err != nil {
		t.Fatalf("SaveAuth() error = %v", err)
	}
	if store.saveCalls != 1 {
		t.Fatalf("save calls = %d, want 1", store.saveCalls)
	}
}

type fakeStore struct {
	saveCalls int
}

func (s *fakeStore) List(context.Context) ([]*cliproxyauth.Auth, error) {
	return nil, nil
}

func (s *fakeStore) Save(context.Context, *cliproxyauth.Auth) (string, error) {
	s.saveCalls++
	return "saved.json", nil
}

func (s *fakeStore) Delete(context.Context, string) error {
	return nil
}

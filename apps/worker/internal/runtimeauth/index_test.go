package runtimeauth

import "testing"

func TestEnsureAuthIndexFromFileNameIsStable(t *testing.T) {
	first := EnsureAuthIndex("user.json", "", "")
	second := EnsureAuthIndex("user.json", "", "")
	if first == "" {
		t.Fatal("expected non-empty auth index")
	}
	if first != second {
		t.Fatalf("index mismatch: %q != %q", first, second)
	}
}

func TestEnsureAuthIndexFallsBackToAPIKeyThenID(t *testing.T) {
	if got := EnsureAuthIndex("", "test-key", ""); got == "" {
		t.Fatal("expected api-key-based index")
	}
	if got := EnsureAuthIndex("", "", "auth-id"); got == "" {
		t.Fatal("expected id-based index")
	}
}

package cache

import (
	"testing"
)

func TestDeletePrefix_MatchingKeys(t *testing.T) {
	m := New()
	defer m.Close()

	m.Put("stats:global", "g", 0)
	m.Put("stats:daily:+08:00", "d", 0)
	m.Put("stats:model", "m", 0)
	m.Put("channels", "ch", 0)

	m.DeletePrefix("stats:")

	// stats keys should be gone
	if _, ok := Get[string](m, "stats:global"); ok {
		t.Error("stats:global should have been deleted")
	}
	if _, ok := Get[string](m, "stats:daily:+08:00"); ok {
		t.Error("stats:daily should have been deleted")
	}
	if _, ok := Get[string](m, "stats:model"); ok {
		t.Error("stats:model should have been deleted")
	}

	// non-matching key should remain
	if v, ok := Get[string](m, "channels"); !ok || *v != "ch" {
		t.Error("channels should still exist")
	}
}

func TestDeletePrefix_NoMatch(t *testing.T) {
	m := New()
	defer m.Close()

	m.Put("channels", "ch", 0)
	m.Put("groups", "gr", 0)

	m.DeletePrefix("stats:")

	if v, ok := Get[string](m, "channels"); !ok || *v != "ch" {
		t.Error("channels should still exist")
	}
	if v, ok := Get[string](m, "groups"); !ok || *v != "gr" {
		t.Error("groups should still exist")
	}
}

func TestDeletePrefix_EmptyStore(t *testing.T) {
	m := New()
	defer m.Close()

	// should not panic on empty store
	m.DeletePrefix("stats:")
}

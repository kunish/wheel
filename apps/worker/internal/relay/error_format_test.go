package relay

import "testing"

func assertStableOpenAIShortCircuit(t *testing.T, sc *ShortCircuit, wantStatus int, wantType, wantMessage string) {
	t.Helper()

	if sc == nil {
		t.Fatal("expected short circuit, got nil")
	}
	if sc.StatusCode != wantStatus {
		t.Fatalf("status = %d, want %d", sc.StatusCode, wantStatus)
	}
	errObj, ok := sc.Body["error"].(map[string]any)
	if !ok {
		t.Fatalf("expected error object, got %#v", sc.Body)
	}
	if got := errObj["type"]; got != wantType {
		t.Fatalf("error.type = %#v, want %q", got, wantType)
	}
	if got := errObj["message"]; got != wantMessage {
		t.Fatalf("error.message = %#v, want %q", got, wantMessage)
	}
	if _, ok := errObj["param"]; !ok {
		t.Fatalf("expected error.param key, got %#v", errObj)
	}
	if errObj["param"] != nil {
		t.Fatalf("expected error.param null, got %#v", errObj["param"])
	}
	if _, ok := errObj["code"]; !ok {
		t.Fatalf("expected error.code key, got %#v", errObj)
	}
	if errObj["code"] != nil {
		t.Fatalf("expected error.code null, got %#v", errObj["code"])
	}
}

func TestRateLimitPlugin_PreHookReturnsStableOpenAIErrorEnvelope(t *testing.T) {
	plugin := NewRateLimitPlugin(func(ctx *RelayContext) RateLimitConfig {
		return RateLimitConfig{RPM: 1}
	})
	ctx := &RelayContext{ApiKeyID: 1, RequestModel: "gpt-4o-mini"}

	if sc := plugin.PreHook(ctx); sc != nil {
		t.Fatalf("expected first request to be allowed, got %#v", sc)
	}

	sc := plugin.PreHook(ctx)
	assertStableOpenAIShortCircuit(t, sc, 429, "rate_limit_error", sc.Body["error"].(map[string]any)["message"].(string))
}

func TestContentFilterPlugin_PreHookReturnsStableOpenAIErrorEnvelope(t *testing.T) {
	t.Run("keyword block", func(t *testing.T) {
		plugin := newContentFilterPlugin(contentFilterConfig{Enabled: true, BlockedKeywords: []string{"forbidden"}})
		sc := plugin.PreHook(&RelayContext{Body: map[string]any{
			"messages": []any{map[string]any{"role": "user", "content": "forbidden text"}},
		}})
		assertStableOpenAIShortCircuit(t, sc, 400, "content_filter_error", "Request blocked by content filter")
	})

	t.Run("input length block", func(t *testing.T) {
		plugin := newContentFilterPlugin(contentFilterConfig{Enabled: true, MaxInputLength: 5})
		sc := plugin.PreHook(&RelayContext{Body: map[string]any{
			"messages": []any{map[string]any{"role": "user", "content": "too long"}},
		}})
		assertStableOpenAIShortCircuit(t, sc, 400, "invalid_request_error", "Input too long: 8 characters (max 5)")
	})
}

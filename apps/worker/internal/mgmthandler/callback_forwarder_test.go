package mgmthandler

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
)

func TestStartCallbackForwarder_ListensAndRedirects(t *testing.T) {
	port := 19871
	target := "http://127.0.0.1:9999/test/callback"

	fw, err := startCallbackForwarder(port, "test", target)
	if err != nil {
		t.Fatalf("startCallbackForwarder: %v", err)
	}
	defer stopCallbackForwarderInstance(port, fw)

	// Make a request that should be redirected
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse // don't follow redirects
		},
	}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/callback?code=abc&state=xyz", port))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusFound)
	}
	loc := resp.Header.Get("Location")
	want := target + "?code=abc&state=xyz"
	if loc != want {
		t.Errorf("Location = %q, want %q", loc, want)
	}
}

func TestStartCallbackForwarder_EvictsPrevious(t *testing.T) {
	port := 19872

	fw1, err := startCallbackForwarder(port, "test1", "http://127.0.0.1:9999/a")
	if err != nil {
		t.Fatalf("startCallbackForwarder #1: %v", err)
	}
	_ = fw1 // will be evicted

	fw2, err := startCallbackForwarder(port, "test2", "http://127.0.0.1:9999/b")
	if err != nil {
		t.Fatalf("startCallbackForwarder #2: %v", err)
	}
	defer stopCallbackForwarderInstance(port, fw2)

	// Verify the second forwarder is active by making a request
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/test", port))
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	loc := resp.Header.Get("Location")
	if loc != "http://127.0.0.1:9999/b" {
		t.Errorf("Location = %q, want target from second forwarder", loc)
	}
}

func TestStopCallbackForwarderInstance_FreesPort(t *testing.T) {
	port := 19873

	fw, err := startCallbackForwarder(port, "test", "http://127.0.0.1:9999/x")
	if err != nil {
		t.Fatalf("startCallbackForwarder: %v", err)
	}
	stopCallbackForwarderInstance(port, fw)

	// Port should be free now; starting again should succeed
	fw2, err := startCallbackForwarder(port, "test2", "http://127.0.0.1:9999/y")
	if err != nil {
		t.Fatalf("startCallbackForwarder after stop: %v", err)
	}
	stopCallbackForwarderInstance(port, fw2)
}

func TestIsWebUIRequest(t *testing.T) {
	tests := []struct {
		name  string
		query string
		want  bool
	}{
		{"empty", "", false},
		{"true", "true", true},
		{"1", "1", true},
		{"yes", "yes", true},
		{"on", "on", true},
		{"TRUE", "TRUE", true},
		{"false", "false", false},
		{"0", "0", false},
		{"random", "random", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &fakeQueryer{val: tt.query}
			if got := isWebUIRequest(mock); got != tt.want {
				t.Errorf("isWebUIRequest(%q) = %v, want %v", tt.query, got, tt.want)
			}
		})
	}
}

type fakeQueryer struct{ val string }

func (f *fakeQueryer) Query(key string) string {
	if key == "is_webui" {
		return f.val
	}
	return ""
}

func TestManagementCallbackURL(t *testing.T) {
	url, err := managementCallbackURL(8787, false, "/codex/callback")
	if err != nil {
		t.Fatal(err)
	}
	if url != "http://127.0.0.1:8787/codex/callback" {
		t.Errorf("url = %q", url)
	}

	url, err = managementCallbackURL(443, true, "google/callback")
	if err != nil {
		t.Fatal(err)
	}
	if url != "https://127.0.0.1:443/google/callback" {
		t.Errorf("url = %q", url)
	}

	_, err = managementCallbackURL(0, false, "/x")
	if err == nil {
		t.Error("expected error for port 0")
	}
}

func TestPollForCallbackFile_Success(t *testing.T) {
	dir := t.TempDir()
	state := "test-state-123"
	provider := "testprov"

	// Register a pending session
	sdkcliproxy.RegisterOAuthSession(state, provider)
	defer sdkcliproxy.CompleteOAuthSession(state)

	// Write the callback file after a short delay
	go func() {
		time.Sleep(100 * time.Millisecond)
		content := fmt.Sprintf(`{"code":"auth-code-xyz","state":"%s"}`, state)
		waitFile := filepath.Join(dir, fmt.Sprintf(".oauth-%s-%s.oauth", provider, state))
		_ = os.WriteFile(waitFile, []byte(content), 0644)
	}()

	result, err := pollForCallbackFile(dir, provider, state)
	if err != nil {
		t.Fatalf("pollForCallbackFile: %v", err)
	}
	if result.Code != "auth-code-xyz" {
		t.Errorf("Code = %q, want auth-code-xyz", result.Code)
	}
	if result.State != state {
		t.Errorf("State = %q, want %s", result.State, state)
	}
}

func TestPollForCallbackFile_Error(t *testing.T) {
	dir := t.TempDir()
	state := "test-state-err"
	provider := "testprov"

	sdkcliproxy.RegisterOAuthSession(state, provider)
	defer sdkcliproxy.CompleteOAuthSession(state)

	// Write callback file with an error
	go func() {
		time.Sleep(100 * time.Millisecond)
		content := fmt.Sprintf(`{"error":"access_denied","state":"%s"}`, state)
		waitFile := filepath.Join(dir, fmt.Sprintf(".oauth-%s-%s.oauth", provider, state))
		_ = os.WriteFile(waitFile, []byte(content), 0644)
	}()

	_, err := pollForCallbackFile(dir, provider, state)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := err.Error(); got != "oauth error: access_denied" {
		t.Errorf("error = %q", got)
	}
}

func TestPollForCallbackFile_StateMismatch(t *testing.T) {
	dir := t.TempDir()
	state := "test-state-mismatch"
	provider := "testprov"

	sdkcliproxy.RegisterOAuthSession(state, provider)
	defer sdkcliproxy.CompleteOAuthSession(state)

	go func() {
		time.Sleep(100 * time.Millisecond)
		content := `{"code":"abc","state":"wrong-state"}`
		waitFile := filepath.Join(dir, fmt.Sprintf(".oauth-%s-%s.oauth", provider, state))
		_ = os.WriteFile(waitFile, []byte(content), 0644)
	}()

	_, err := pollForCallbackFile(dir, provider, state)
	if err == nil {
		t.Fatal("expected error")
	}
	if !contains(err.Error(), "state mismatch") {
		t.Errorf("error = %q, want state mismatch", err.Error())
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsImpl(s, substr))
}

func containsImpl(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

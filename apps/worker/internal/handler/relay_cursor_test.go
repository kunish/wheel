package handler

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/kunish/wheel/apps/worker/internal/relay"
)

func TestParseCursorCredentials_JSON(t *testing.T) {
	t.Parallel()
	raw := `{"accessToken":"tok","machineId":"m1","macMachineId":"m2","clientVersion":"2.0.0"}`
	c, err := parseCursorCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "tok" || c.MachineID != "m1" || c.MacMachineID != "m2" || c.ClientVersion != "2.0.0" {
		t.Fatalf("%+v", c)
	}
}

func TestParseCursorCredentials_PlainToken(t *testing.T) {
	t.Parallel()
	c, err := parseCursorCredentials("plain-secret")
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "plain-secret" {
		t.Fatalf("%+v", c)
	}
}

func TestParseCursorCredentials_TokensWrapper(t *testing.T) {
	t.Parallel()
	raw := `{"tokens":[{"accessToken":"tok2","machineId":"m1","macMachineId":"m2","clientVersion":"2.1.0"}]}`
	c, err := parseCursorCredentials(raw)
	if err != nil {
		t.Fatal(err)
	}
	if c.AccessToken != "tok2" || c.MachineID != "m1" || c.MacMachineID != "m2" || c.ClientVersion != "2.1.0" {
		t.Fatalf("%+v", c)
	}
}

func TestCursorModelIDsFromUsableModelsJSON(t *testing.T) {
	t.Parallel()
	got, err := cursorModelIDsFromUsableModelsJSON([]byte(
		`{"models":["composer-2",{"modelId":"claude-4.5-sonnet"},{"model_id":"gemini-3-flash"}]}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"composer-2", "claude-4.5-sonnet", "gemini-3-flash"}
	if len(got) != len(want) {
		t.Fatalf("got %v want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v want %v", got, want)
		}
	}

	got2, err := cursorModelIDsFromUsableModelsJSON([]byte(
		`{"result":{"usableModels":[{"name":"kimi-k2.5"}]}}`,
	))
	if err != nil {
		t.Fatal(err)
	}
	if len(got2) != 1 || got2[0] != "kimi-k2.5" {
		t.Fatalf("got %v", got2)
	}
}

func TestRequestTypeSupportedByCursor(t *testing.T) {
	t.Parallel()
	if !requestTypeSupportedByCursor(relay.RequestTypeChat, false) {
		t.Fatal("expected chat completions")
	}
	if !requestTypeSupportedByCursor(relay.RequestTypeAnthropicMsg, true) {
		t.Fatal("expected anthropic /v1/messages when inbound")
	}
	if requestTypeSupportedByCursor(relay.RequestTypeAnthropicMsg, false) {
		t.Fatal("unexpected anthropic without inbound flag")
	}
}

func TestCursorMessagesToPrompt(t *testing.T) {
	t.Parallel()
	body := map[string]any{
		"messages": []any{
			map[string]any{"role": "system", "content": "You are helpful."},
			map[string]any{"role": "user", "content": "Hi"},
		},
	}
	s, err := cursorMessagesToPrompt(body)
	if err != nil {
		t.Fatal(err)
	}
	for _, needle := range []string{"<system>", "Hi", "helpful"} {
		if !strings.Contains(s, needle) {
			t.Fatalf("expected %q in %q", needle, s)
		}
	}
}

func TestEncodeCursorFrame(t *testing.T) {
	t.Parallel()
	b, err := encodeCursorFrame(map[string]any{"a": 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(b) < 6 || b[0] != 0 {
		t.Fatalf("frame: %v", b[:min(10, len(b))])
	}
	n := int(b[1])<<24 | int(b[2])<<16 | int(b[3])<<8 | int(b[4])
	payload := b[5:]
	if n != len(payload) {
		t.Fatalf("len %d payload %d", n, len(payload))
	}
	var obj map[string]any
	if err := json.Unmarshal(payload, &obj); err != nil {
		t.Fatal(err)
	}
	if obj["a"].(float64) != 1 {
		t.Fatalf("%v", obj)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

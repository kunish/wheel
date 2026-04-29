package handler

import (
	"encoding/json"
	"testing"
)

func TestNamedJSONArrayCoercesRawMessage(t *testing.T) {
	raw := json.RawMessage(`[{"name":"x","input_schema":{"type":"object"}}]`)
	body := map[string]any{"tools": raw}
	got := namedJSONArray(body, "tools")
	if len(got) != 1 {
		t.Fatalf("got %v", got)
	}
	if !cursorBodyDeclaresTools(body) {
		t.Fatal("expected declares tools")
	}
}

func TestCursorBodyDeclaresLegacyFunctions(t *testing.T) {
	body := map[string]any{
		"functions": []any{map[string]any{"name": "fn", "parameters": map[string]any{"type": "object"}}},
	}
	if !cursorBodyDeclaresTools(body) {
		t.Fatal("functions should count as tools")
	}
}

func TestCursorRelayShouldUseComChatBridgeOriginal(t *testing.T) {
	p := &relayAttemptParams{
		Body: map[string]any{"model": "m", "messages": []any{}},
		BridgeOriginalBody: map[string]any{
			"model": "m",
			"tools": []any{map[string]any{"name": "Read", "input_schema": map[string]any{}}},
		},
	}
	if !cursorRelayShouldUseComChat(p) {
		t.Fatal("expected com chat when BridgeOriginalBody has tools")
	}
}

func TestCursorRelayShouldUseComChatInboundSnapshotAfterBodyStrip(t *testing.T) {
	stripped := map[string]any{
		"model":    "m",
		"messages": []any{map[string]any{"role": "user", "content": "hi"}},
	}
	snap := map[string]any{
		"model": "m",
		"tools": []any{map[string]any{"name": "Read", "input_schema": map[string]any{"type": "object"}}},
		"messages": []any{
			map[string]any{"role": "user", "content": "hi"},
		},
	}
	p := &relayAttemptParams{
		Body:               stripped,
		BridgeOriginalBody: nil,
		InboundSnapshot:    snap,
	}
	if !cursorRelayShouldUseComChat(p) {
		t.Fatal("expected com chat when InboundSnapshot still has tools")
	}
}

func TestNamedJSONArrayParsesToolsJSONString(t *testing.T) {
	body := map[string]any{"tools": `[{"name":"x","input_schema":{"type":"object"}}]`}
	if len(namedJSONArray(body, "tools")) != 1 {
		t.Fatalf("got %#v", namedJSONArray(body, "tools"))
	}
}

func TestRelayHeuristicToolsUnicodeEscapedKey(t *testing.T) {
	raw := []byte(`{\u0022model\u0022:\u0022m\u0022,\u0022tools\u0022:[{\u0022name\u0022:\u0022x\u0022}]}`)
	if !relayHeuristicToolsInJSON(raw) {
		t.Fatal("expected escaped tools key to match")
	}
}

func TestCursorRelayShouldUseComChatRawWireJSON(t *testing.T) {
	raw := []byte(`{"model":"m","messages":[],"tools":[{"name":"Read","input_schema":{"type":"object"}}]}`)
	p := &relayAttemptParams{
		Body:               map[string]any{"model": "m", "messages": []any{}},
		BridgeOriginalBody: nil,
		InboundSnapshot:    nil,
		InboundRawJSON:     raw,
	}
	if !cursorRelayShouldUseComChat(p) {
		t.Fatal("expected com chat from InboundRawJSON tools array pattern")
	}
	if !relayHeuristicToolsInJSON(raw) {
		t.Fatal("heuristic should detect tools")
	}
}

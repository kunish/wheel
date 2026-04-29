package handler

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"golang.org/x/net/http2"
)

// Cursor channel — OpenAI-compatible bridge via https://cursor.com/api/chat (web chat API).
// Optional: GetUsableModels uses Cursor api2 Connect (HTTP/2) for model listing only.

const (
	cursorDefaultBaseURL      = "https://api2.cursor.sh"
	cursorDefaultClientVer    = "2.6.20"
	cursorDefaultDisplayModel = "claude-4.5-sonnet"
	cursorAgentModelsPath     = "/agent.v1.AgentService/GetUsableModels"
)

// Wire-level detection: some intermediaries produce JSON that round-trips oddly into map[string]any.
var (
	cursorWireToolsArrayKey = regexp.MustCompile(`"tools"\s*:\s*\[`)
	cursorWireFunctionsKey  = regexp.MustCompile(`"functions"\s*:\s*\[`)
)

// relayHeuristicToolsInJSON is a last-resort scan of raw JSON for structured tool fields.
func relayHeuristicToolsInJSON(b []byte) bool {
	if len(b) < 24 {
		return false
	}
	if bytes.Contains(b, []byte(`"tool_calls"`)) ||
		bytes.Contains(b, []byte(`"tool_use"`)) ||
		bytes.Contains(b, []byte(`"tool_result"`)) {
		return true
	}
	if bytes.Contains(b, []byte(`"role":"tool"`)) || bytes.Contains(b, []byte(`"role": "tool"`)) {
		return true
	}
	if bytes.Contains(b, []byte(`"tool_choice"`)) {
		return true
	}
	if cursorWireToolsArrayKey.Match(b) || cursorWireFunctionsKey.Match(b) {
		return true
	}
	// Some JSON encoders emit Unicode-escaped quotes around keys.
	if bytes.Contains(b, []byte(`\u0022tools\u0022`)) && bytes.Contains(b, []byte(`[`)) {
		return true
	}
	if bytes.Contains(b, []byte(`\u0022functions\u0022`)) && bytes.Contains(b, []byte(`[`)) {
		return true
	}
	// Claude Code MCP plugin tools use stable name prefixes in definitions.
	if bytes.Contains(b, []byte(`"mcp__`)) {
		return true
	}
	if bytes.Contains(b, []byte(`"input_schema"`)) && bytes.Contains(b, []byte(`"name"`)) {
		return true
	}
	if bytes.Contains(b, []byte(`parallel_tool_calls`)) {
		return true
	}
	if bytes.Contains(b, []byte(`tool_resources`)) || bytes.Contains(b, []byte(`"tool_resources"`)) {
		return true
	}
	if bytes.Contains(b, []byte(`"function_call"`)) {
		return true
	}
	return false
}

func cursorLegacyAgentDisabledMessage() string {
	return "Wheel does not call Cursor api2 Agent Run (ConnectRPC). Cursor channels use https://cursor.com/api/chat only. " +
		"If you still see Cursor's own “client-side tools / plain text relay” message, you are not hitting this worker's relay code path: " +
		"use channel type Cursor (37), point Claude Code BASE_URL at this Wheel /v1, redeploy the latest apps/worker image, and remove any intermediate proxy that forwards to api2 or a third-party plain-text Cursor relay."
}

// cursorExhaustionHintAfterPlainTextRelayError appends context when upstream returns Cursor Agent / plain-text-relay wording
// (Wheel does not emit that text; it comes from api2 or a third-party OpenAI facade in front of Cursor).
func cursorExhaustionHintAfterPlainTextRelayError(lastError string) string {
	if lastError == "" {
		return ""
	}
	lr := strings.ToLower(lastError)
	if !strings.Contains(lr, "client-side tools") &&
		!strings.Contains(lr, "plain text relay") &&
		!strings.Contains(lr, "plain text to") &&
		!strings.Contains(lr, "cursor agent api") {
		return ""
	}
	return " [Wheel: that message is from Cursor Agent or a plain-text Cursor relay, not from Wheel's Cursor (37) bridge. " +
		"In the admin UI set the channel type to Cursor (37) with a Cursor token; do not use an OpenAI-compatible channel whose base URL points at api2 or another Cursor OpenAI façade. " +
		"Unset CURSOR_NO_COM_CHAT_FALLBACK unless you intentionally disabled the HTTP client for cursor.com/api/chat.]"
}

// cursorToolsUnsupportedMessage is returned when tool/workflows are detected but CursorRelay.HTTPClient is not set.
const cursorToolsUnsupportedMessage = "Cursor tool calling requires Wheel HTTP client configuration (internal error). Rebuild/restart the worker, or report this issue."

// namedJSONArray returns a JSON array from body[key] even when the dynamic type is not []any
// (e.g. json.RawMessage, typed slices, or alternate decoders). Empty arrays return nil.
func namedJSONArray(body map[string]any, key string) []any {
	if body == nil {
		return nil
	}
	v, ok := body[key]
	if !ok || v == nil {
		return nil
	}
	switch x := v.(type) {
	case []any:
		if len(x) == 0 {
			return nil
		}
		return x
	case string:
		// Some proxies stringify the tools JSON array; parse it.
		s := strings.TrimSpace(x)
		if len(s) > 1 && s[0] == '[' {
			var arr []any
			if json.Unmarshal([]byte(s), &arr) == nil && len(arr) > 0 {
				return arr
			}
		}
		return nil
	default:
		raw, err := json.Marshal(x)
		if err != nil {
			return nil
		}
		var arr []any
		if err := json.Unmarshal(raw, &arr); err != nil || len(arr) == 0 {
			return nil
		}
		return arr
	}
}

// cursorBodyDeclaresTools reports whether the request declares OpenAI tools and/or legacy "functions".
func cursorBodyDeclaresTools(body map[string]any) bool {
	return len(namedJSONArray(body, "tools")) > 0 || len(namedJSONArray(body, "functions")) > 0
}

// bodyDeclaresToolChoice is true when the client set tool_choice / routing wants tools (Anthropic or OpenAI).
func bodyDeclaresToolChoice(body map[string]any) bool {
	if body == nil {
		return false
	}
	tc, ok := body["tool_choice"]
	if !ok || tc == nil {
		return false
	}
	switch x := tc.(type) {
	case string:
		s := strings.TrimSpace(strings.ToLower(x))
		return s != "" && s != "none"
	case map[string]any:
		return len(x) > 0
	default:
		return true
	}
}

func toolCallsInOpenAIMessage(msg map[string]any) bool {
	tc, ok := msg["tool_calls"]
	if !ok || tc == nil {
		return false
	}
	if arr, ok := tc.([]any); ok && len(arr) > 0 {
		return true
	}
	raw, err := json.Marshal(tc)
	if err != nil {
		return false
	}
	var arr []any
	return json.Unmarshal(raw, &arr) == nil && len(arr) > 0
}

// cursorBodyImpliesClientTooling is true for top-level tools/functions or an OpenAI chat history that uses tools.
func cursorBodyImpliesClientTooling(body map[string]any) bool {
	if bodyDeclaresToolChoice(body) {
		return true
	}
	if cursorBodyDeclaresTools(body) {
		return true
	}
	for _, m := range namedJSONArray(body, "messages") {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		if role, _ := msg["role"].(string); role == "tool" {
			return true
		}
		if toolCallsInOpenAIMessage(msg) {
			return true
		}
	}
	return false
}

// anthropicBodyImpliesTooling detects tool_use / tool_result in Anthropic-shaped bodies (BridgeOriginalBody).
func anthropicBodyImpliesTooling(body map[string]any) bool {
	if body == nil {
		return false
	}
	if bodyDeclaresToolChoice(body) {
		return true
	}
	if cursorBodyDeclaresTools(body) {
		return true
	}
	for _, m := range namedJSONArray(body, "messages") {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		c := msg["content"]
		arr, ok := c.([]any)
		if !ok {
			continue
		}
		for _, x := range arr {
			b, ok := x.(map[string]any)
			if !ok {
				continue
			}
			switch typ, _ := b["type"].(string); typ {
			case "tool_use", "tool_result":
				return true
			}
		}
	}
	return false
}

// cursorRelayShouldUseComChat is true when we must use cursor.com/api/chat (tools / function calling).
func cursorRelayShouldUseComChat(p *relayAttemptParams) bool {
	if p == nil {
		return false
	}
	if cursorBodyImpliesClientTooling(p.Body) {
		return true
	}
	if p.BridgeOriginalBody != nil && anthropicBodyImpliesTooling(p.BridgeOriginalBody) {
		return true
	}
	if p.InboundSnapshot != nil {
		if cursorBodyImpliesClientTooling(p.InboundSnapshot) || anthropicBodyImpliesTooling(p.InboundSnapshot) {
			return true
		}
	}
	if len(p.InboundRawJSON) > 0 && relayHeuristicToolsInJSON(p.InboundRawJSON) {
		return true
	}
	return false
}

var (
	cursorHTTP2Once   sync.Once
	cursorHTTP2Client *http.Client
)

func cursorSharedH2Client() *http.Client {
	cursorHTTP2Once.Do(func() {
		t := &http.Transport{}
		_ = http2.ConfigureTransport(t)
		cursorHTTP2Client = &http.Client{Transport: t, Timeout: 0}
	})
	return cursorHTTP2Client
}

// CursorRelay implements Cursor web-chat relay and auxiliary Cursor API calls.
type CursorRelay struct {
	// HTTPClient is used for cursor.com/api/chat when the request implies client-side tools.
	// Must be set by the worker (see main); if nil and tools are implied, requests fail with an explicit error.
	HTTPClient *http.Client
}

// NewCursorRelay returns a stateless Cursor relay.
func NewCursorRelay() *CursorRelay { return &CursorRelay{} }

// cursorRelayComChatHTTPClient picks the HTTP client for cursor.com/api/chat (handler override or relay).
func cursorRelayComChatHTTPClient(comChatHTTP *http.Client, r *CursorRelay) *http.Client {
	if comChatHTTP != nil {
		return comChatHTTP
	}
	if r != nil && r.HTTPClient != nil {
		return r.HTTPClient
	}
	if strings.TrimSpace(os.Getenv("CURSOR_NO_COM_CHAT_FALLBACK")) == "1" {
		return nil
	}
	return cursorComChatFallbackHTTPClient()
}

// cursorRelayRouteComChat is true whenever we have an HTTP client for cursor.com/api/chat.
// Agent (api2 ConnectRPC) is not used for Cursor channels when this client exists: Claude Code and
// similar clients embed tool instructions in prompts that Agent rejects as “client-side tools”, so
// we always prefer the web chat bridge. CURSOR_USE_AGENT no longer switches routing when a client is set.
func cursorRelayRouteComChat(comChatHTTP *http.Client, r *CursorRelay, toolsLike bool) bool {
	_ = toolsLike // reserved for call-site symmetry / future limits
	return cursorRelayComChatHTTPClient(comChatHTTP, r) != nil
}

type cursorCredentials struct {
	AccessToken   string `json:"accessToken"`
	MachineID     string `json:"machineId"`
	MacMachineID  string `json:"macMachineId"`
	ClientVersion string `json:"clientVersion"`
}

func parseCursorCredentials(raw string) (cursorCredentials, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return cursorCredentials{}, fmt.Errorf("empty channel key")
	}
	if raw[0] == '{' {
		var c cursorCredentials
		if err := json.Unmarshal([]byte(raw), &c); err != nil {
			return cursorCredentials{}, fmt.Errorf("cursor key JSON: %w", err)
		}
		c.AccessToken = strings.TrimSpace(c.AccessToken)
		// cursoride2api token.json: { "tokens": [ { "accessToken", ... }, ... ] }
		if c.AccessToken == "" {
			var wrap struct {
				Tokens []cursorCredentials `json:"tokens"`
			}
			if err := json.Unmarshal([]byte(raw), &wrap); err == nil && len(wrap.Tokens) > 0 {
				c = wrap.Tokens[0]
				c.AccessToken = strings.TrimSpace(c.AccessToken)
				c.MachineID = strings.TrimSpace(c.MachineID)
				c.MacMachineID = strings.TrimSpace(c.MacMachineID)
				c.ClientVersion = strings.TrimSpace(c.ClientVersion)
			}
		}
		if c.AccessToken == "" {
			return cursorCredentials{}, fmt.Errorf("cursor key: accessToken is required")
		}
		if strings.TrimSpace(c.ClientVersion) == "" {
			c.ClientVersion = cursorDefaultClientVer
		}
		return c, nil
	}
	return cursorCredentials{
		AccessToken:   raw,
		ClientVersion: cursorDefaultClientVer,
	}, nil
}

// cursorModelIDsFromUsableModelsJSON extracts model ids from GetUsableModels JSON.
// The API usually returns { "models": [ { "modelId": "..." } ] } but some responses
// use string ids, snake_case fields, or a Connect "result" wrapper.
func cursorModelIDsFromUsableModelsJSON(body []byte) ([]string, error) {
	var top map[string]any
	if err := json.Unmarshal(body, &top); err != nil {
		return nil, err
	}
	if inner, ok := top["result"].(map[string]any); ok {
		top = inner
	}
	rawModels, _ := top["models"].([]any)
	if rawModels == nil {
		rawModels, _ = top["usableModels"].([]any)
	}
	out := make([]string, 0, len(rawModels))
	seen := map[string]struct{}{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id == "" {
			return
		}
		if _, ok := seen[id]; ok {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, rm := range rawModels {
		switch v := rm.(type) {
		case string:
			add(v)
		case map[string]any:
			var id string
			for _, key := range []string{"modelId", "model_id", "id", "name", "modelName", "model"} {
				if s, ok := v[key].(string); ok && strings.TrimSpace(s) != "" {
					id = strings.TrimSpace(s)
					break
				}
				if n, ok := v[key].(float64); ok {
					id = fmt.Sprintf("%.0f", n)
					break
				}
			}
			add(id)
		}
	}
	return out, nil
}

// cursorFallbackUsableModelIDs returns known Cursor upstream model ids from the local
// mapping table when GetUsableModels returns an empty list (still HTTP 200).
func cursorFallbackUsableModelIDs() []string {
	m := cursorModelMapping()
	seen := make(map[string]struct{}, len(m))
	out := make([]string, 0, len(m))
	for _, v := range m {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

func cursorModelMapping() map[string]string {
	return map[string]string{
		"gpt-4":             "composer-2",
		"gpt-4o":            "composer-2",
		"gpt-4o-mini":       "composer-2-fast",
		"gpt-4-turbo":       "composer-2",
		"gpt-3.5-turbo":     "composer-1.5",
		"claude-3-opus":     "claude-4.6-opus-high",
		"claude-3-sonnet":   "claude-4.6-sonnet-medium",
		"claude-3.5-sonnet": "claude-4.5-sonnet",
		// Group-style aliases (hyphen minor / name-major ordering).
		"claude-opus-4-6":          "claude-4.6-opus-high",
		"claude-opus-4.6":          "claude-4.6-opus-high",
		"claude-sonnet-4-6":        "claude-4.6-sonnet-medium",
		"claude-sonnet-4.6":        "claude-4.6-sonnet-medium",
		"claude-4-6-opus-high":     "claude-4.6-opus-high",
		"claude-4.6-opus-high":     "claude-4.6-opus-high",
		"claude-4-6-sonnet-medium": "claude-4.6-sonnet-medium",
		"claude-4.6-sonnet-medium": "claude-4.6-sonnet-medium",
		"claude-sonnet-4-5":        "claude-4.5-sonnet",
		"claude-sonnet-4.5":        "claude-4.5-sonnet",
		"gemini-pro":               "gemini-3.1-pro",
		"composer-2":               "composer-2",
		"composer-2-fast":          "composer-2-fast",
		"composer-1.5":             "composer-1.5",
		"default":                  "default",
	}
}

func mapCursorUpstreamModel(requested string) string {
	requested = strings.TrimSpace(requested)
	if requested == "" {
		return cursorDefaultDisplayModel
	}
	mapping := cursorModelMapping()
	if m, ok := mapping[requested]; ok {
		return m
	}
	norm := normalizeClaudeHyphenVersions(requested)
	if norm != requested {
		if m, ok := mapping[norm]; ok {
			return m
		}
	}
	return requested
}

// cursorChecksum matches cursoride2api generateChecksum (Date.now()/1e6 based).
func cursorChecksum(machineID, macMachineID string) string {
	k := 165
	t := int(time.Now().UnixMilli() / 1_000_000)
	b := []byte{
		byte(t >> 40), byte(t >> 32), byte(t >> 24),
		byte(t >> 16), byte(t >> 8), byte(t),
	}
	for i := 0; i < len(b); i++ {
		b[i] = byte((int(b[i]^byte(k)) + (i % 256)) & 0xff)
		k = int(b[i])
	}
	prefix := base64.StdEncoding.EncodeToString(b)
	if strings.TrimSpace(macMachineID) != "" {
		return prefix + machineID + "/" + macMachineID
	}
	return prefix + machineID
}

func encodeCursorFrame(obj map[string]any) ([]byte, error) {
	jsonBuf, err := json.Marshal(obj)
	if err != nil {
		return nil, err
	}
	frame := make([]byte, 5+len(jsonBuf))
	frame[0] = 0
	frame[1] = byte(len(jsonBuf) >> 24)
	frame[2] = byte(len(jsonBuf) >> 16)
	frame[3] = byte(len(jsonBuf) >> 8)
	frame[4] = byte(len(jsonBuf))
	copy(frame[5:], jsonBuf)
	return frame, nil
}

func cursorMessagesToPrompt(body map[string]any) (string, error) {
	raw, ok := body["messages"]
	if !ok {
		return "", fmt.Errorf("messages required")
	}
	arr, ok := raw.([]any)
	if !ok || len(arr) == 0 {
		return "", fmt.Errorf("messages required")
	}
	var b strings.Builder
	for _, item := range arr {
		msg, ok := item.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		if role == "" {
			role = "user"
		}
		var content string
		switch c := msg["content"].(type) {
		case string:
			content = c
		case []any:
			for _, p := range c {
				part, ok := p.(map[string]any)
				if !ok {
					continue
				}
				if typ, _ := part["type"].(string); typ == "text" {
					if t, _ := part["text"].(string); t != "" {
						content += t + "\n"
					}
				}
			}
			content = strings.TrimSpace(content)
		default:
		}
		if content == "" {
			continue
		}
		if b.Len() > 0 {
			b.WriteString("\n\n")
		}
		switch role {
		case "system":
			fmt.Fprintf(&b, "<system>\n%s\n</system>", content)
		case "assistant":
			fmt.Fprintf(&b, "<assistant>\n%s\n</assistant>", content)
		default:
			b.WriteString(content)
		}
	}
	out := strings.TrimSpace(b.String())
	if out == "" {
		return "", fmt.Errorf("empty prompt from messages")
	}
	return out, nil
}

func cursorApplyCustomHeaders(req *http.Request, ch *types.Channel) {
	if req == nil || ch == nil {
		return
	}
	for _, h := range ch.CustomHeader {
		k := strings.TrimSpace(h.Key)
		if k == "" {
			continue
		}
		req.Header.Set(k, h.Value)
	}
}

func (r *CursorRelay) cursorChannelBaseURL(ch *types.Channel) string {
	if ch == nil || len(ch.BaseUrls) == 0 {
		return cursorDefaultBaseURL
	}
	u := strings.TrimSpace(ch.BaseUrls[0].URL)
	if u == "" {
		return cursorDefaultBaseURL
	}
	return strings.TrimRight(u, "/")
}

// ProxyNonStreaming returns an OpenAI chat.completion JSON object via cursor.com/api/chat.
func (r *CursorRelay) ProxyNonStreaming(
	ctx context.Context,
	ch *types.Channel,
	rawKey string,
	requestModel string,
	cursorModel string,
	body map[string]any,
	anthropicInbound bool,
	geminiNative bool,
	comChatHTTP *http.Client,
) (*relay.ProxyResult, error) {
	if r == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}
	toolsLike := cursorBodyImpliesClientTooling(body)
	if cursorRelayRouteComChat(comChatHTTP, r, toolsLike) {
		client := cursorRelayComChatHTTPClient(comChatHTTP, r)
		if client == nil {
			return nil, &relay.ProxyError{Message: cursorToolsUnsupportedMessage, StatusCode: http.StatusInternalServerError}
		}
		if geminiNative && toolsLike {
			return nil, &relay.ProxyError{
				Message:    "Cursor channel: Gemini native requests with tools must use a non-Cursor provider",
				StatusCode: http.StatusNotImplemented,
			}
		}
		return cursorComChatProxyResult(ctx, client, cred.AccessToken, body, requestModel, cursorModel, anthropicInbound)
	}
	msg := cursorLegacyAgentDisabledMessage()
	if cursorRelayComChatHTTPClient(comChatHTTP, r) == nil {
		msg = "No HTTP client for cursor.com/api/chat. Unset CURSOR_NO_COM_CHAT_FALLBACK or set RelayHandler/CursorRelay HTTPClient in main. " + msg
	}
	return nil, &relay.ProxyError{Message: msg, StatusCode: http.StatusBadGateway}
}

func cursorChatCompletionID() string {
	s := strings.ReplaceAll(uuid.NewString(), "-", "")
	if len(s) > 24 {
		s = s[:24]
	}
	return "chatcmpl-" + s
}

// cursorStreamOpenAIMeta holds a stable id/created timestamp for one streamed completion.
type cursorStreamOpenAIMeta struct {
	CompletionID string
	Created      int64
	Model        string
}

func newCursorStreamOpenAIMeta(requestedModel string) cursorStreamOpenAIMeta {
	return cursorStreamOpenAIMeta{
		CompletionID: cursorChatCompletionID(),
		Created:      time.Now().Unix(),
		Model:        requestedModel,
	}
}

func (m cursorStreamOpenAIMeta) openAIChunkMap(delta string, finishReason *string) map[string]any {
	deltaMap := map[string]any{}
	if delta != "" {
		deltaMap["content"] = delta
	}
	return map[string]any{
		"id":      m.CompletionID,
		"object":  "chat.completion.chunk",
		"created": m.Created,
		"model":   m.Model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         deltaMap,
			"finish_reason": finishReason,
		}},
	}
}

func (m cursorStreamOpenAIMeta) openAIChunkJSON(delta string, finishReason *string) []byte {
	b, _ := json.Marshal(m.openAIChunkMap(delta, finishReason))
	return b
}

func (m cursorStreamOpenAIMeta) sseRole() []byte {
	ch := map[string]any{
		"id":      m.CompletionID,
		"object":  "chat.completion.chunk",
		"created": m.Created,
		"model":   m.Model,
		"choices": []any{map[string]any{
			"index":         0,
			"delta":         map[string]any{"role": "assistant", "content": ""},
			"finish_reason": nil,
		}},
	}
	b, _ := json.Marshal(ch)
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data: %s\n\n", b)
	return buf.Bytes()
}

func (m cursorStreamOpenAIMeta) sseData(delta string, finishReason *string) []byte {
	b, _ := json.Marshal(m.openAIChunkMap(delta, finishReason))
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "data: %s\n\n", b)
	return buf.Bytes()
}

func cursorOpenAIChatCompletionResponse(requestedModel, text string, inTok, outTok int) map[string]any {
	id := cursorChatCompletionID()
	return map[string]any{
		"id":      id,
		"object":  "chat.completion",
		"created": time.Now().Unix(),
		"model":   requestedModel,
		"choices": []any{map[string]any{
			"index": 0,
			"message": map[string]any{
				"role":    "assistant",
				"content": text,
			},
			"finish_reason": "stop",
		}},
		"usage": map[string]any{
			"prompt_tokens":     inTok,
			"completion_tokens": outTok,
			"total_tokens":      inTok + outTok,
		},
	}
}

// ProxyStreaming streams completion chunks to w.
// Default: OpenAI chat.completion.chunk SSE. With anthropicInbound or geminiNative,
// applies the same conversions as the generic relay (OpenAI SSE → Anthropic / Gemini stream JSON).
func (r *CursorRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	ch *types.Channel,
	rawKey string,
	requestModel string,
	cursorModel string,
	body map[string]any,
	anthropicInbound bool,
	geminiNative bool,
	comChatHTTP *http.Client,
) (*relay.StreamCompleteInfo, error) {
	if r == nil {
		return nil, &relay.ProxyError{Message: "cursor relay not configured", StatusCode: http.StatusInternalServerError}
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}
	toolsLike := cursorBodyImpliesClientTooling(body)
	if cursorRelayRouteComChat(comChatHTTP, r, toolsLike) {
		if geminiNative && toolsLike {
			return nil, &relay.ProxyError{Message: "Cursor channel: Gemini native streaming with client tools is not supported", StatusCode: http.StatusNotImplemented}
		}
		client := cursorRelayComChatHTTPClient(comChatHTTP, r)
		if client == nil {
			return nil, &relay.ProxyError{Message: cursorToolsUnsupportedMessage, StatusCode: http.StatusInternalServerError}
		}
		return cursorComChatStreamProxy(w, ctx, client, cred.AccessToken, body, requestModel, cursorModel, anthropicInbound)
	}
	msg := cursorLegacyAgentDisabledMessage()
	if cursorRelayComChatHTTPClient(comChatHTTP, r) == nil {
		msg = "No HTTP client for cursor.com/api/chat. Unset CURSOR_NO_COM_CHAT_FALLBACK or set RelayHandler/CursorRelay HTTPClient in main. " + msg
	}
	return nil, &relay.ProxyError{Message: msg, StatusCode: http.StatusBadGateway}
}

// FetchUsableModels returns model IDs from Cursor GetUsableModels (optional; for “fetch models” UI).
func (r *CursorRelay) FetchUsableModels(ctx context.Context, ch *types.Channel, rawKey string) ([]string, error) {
	if r == nil {
		return nil, fmt.Errorf("cursor relay not configured")
	}
	cred, err := parseCursorCredentials(rawKey)
	if err != nil {
		return nil, err
	}
	base := r.cursorChannelBaseURL(ch)
	url := base + cursorAgentModelsPath
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("connect-protocol-version", "1")
	req.Header.Set("Authorization", "Bearer "+cred.AccessToken)
	req.Header.Set("x-cursor-checksum", cursorChecksum(cred.MachineID, cred.MacMachineID))
	req.Header.Set("x-cursor-client-version", cred.ClientVersion)
	req.Header.Set("x-request-id", uuid.NewString())
	cursorApplyCustomHeaders(req, ch)

	resp, err := cursorSharedH2Client().Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cursor models %d: %s", resp.StatusCode, string(b))
	}
	out, err := cursorModelIDsFromUsableModelsJSON(b)
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		out = cursorFallbackUsableModelIDs()
	}
	return out, nil
}

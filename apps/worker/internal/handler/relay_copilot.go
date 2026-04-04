package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	"github.com/uptrace/bun"
)

// copilotAPITokenURL is the endpoint for exchanging GitHub tokens for Copilot API tokens.
const (
	copilotRelayBaseURL       = "https://api.githubcopilot.com"
	copilotRelayTokenCacheTTL = 25 * time.Minute
	copilotRelayTokenBuffer   = 5 * time.Minute
	copilotRelayUserAgent     = "GitHubCopilotChat/0.35.0"
	copilotRelayEditorVersion = "vscode/1.107.0"
	copilotRelayPluginVersion = "copilot-chat/0.35.0"
	copilotRelayIntegrationID = "vscode-chat"
	copilotRelayOpenAIIntent  = "conversation-edits"
	copilotRelayGitHubAPIVer  = "2025-04-01"

	// copilotMaxScannerBufferSize is the maximum buffer size for SSE scanning (20MB).
	copilotMaxScannerBufferSize = 20_971_520

	// copilotChatPath and copilotResponsesPath are the upstream API paths.
	copilotChatPath      = "/chat/completions"
	copilotResponsesPath = "/responses"
)

// copilotUnsupportedBetas lists beta headers that are Anthropic-specific and
// must not be forwarded to GitHub Copilot.
var copilotUnsupportedBetas = []string{
	"context-1m-2025-08-07",
}

// dataPrefix is the SSE data line prefix.
var dataPrefix = []byte("data: ")

// copilotCachedToken holds a cached Copilot API token.
type copilotCachedToken struct {
	token       string
	apiEndpoint string
	expiresAt   time.Time
}

// CopilotRelay handles Copilot channel requests internally using direct API calls
// instead of proxying through an external runtime process.
type CopilotRelay struct {
	db *bun.DB

	// tokenMu protects the token cache.
	tokenMu    sync.RWMutex
	tokenCache map[string]*copilotCachedToken // key: github access_token -> cached API token
}

// NewCopilotRelay creates a new CopilotRelay with the given DB for auth file lookup.
func NewCopilotRelay(db *bun.DB) *CopilotRelay {
	return &CopilotRelay{
		db:         db,
		tokenCache: make(map[string]*copilotCachedToken),
	}
}

// ResolveAccessToken maps a channel key (AuthIndex hash) to the actual GitHub access_token
// by loading auth files for the given channel from the database.
func (cr *CopilotRelay) ResolveAccessToken(ctx context.Context, channelID int, channelKey string) (string, error) {
	items, err := dal.ListCodexAuthFiles(ctx, cr.db, channelID)
	if err != nil {
		return "", fmt.Errorf("load copilot auth files: %w", err)
	}

	for _, item := range items {
		if item.Disabled {
			continue
		}
		// Recompute AuthIndex from the managed file name to match against the channel key.
		managedName := codexruntime.ManagedAuthFileName(item.ChannelID, item.Name)
		authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")
		if authIndex == "" {
			authIndex = item.Name
		}
		if authIndex != channelKey {
			continue
		}
		// Parse content to extract access_token.
		var raw map[string]any
		if err := json.Unmarshal([]byte(item.Content), &raw); err != nil {
			return "", fmt.Errorf("parse copilot auth file content: %w", err)
		}
		if token, ok := raw["access_token"].(string); ok && strings.TrimSpace(token) != "" {
			return strings.TrimSpace(token), nil
		}
		return "", fmt.Errorf("copilot auth file %q has no access_token", item.Name)
	}

	return "", fmt.Errorf("no copilot auth file matches channel key %q", channelKey)
}

// ensureAPIToken gets or refreshes the Copilot API token for the given GitHub access token.
func (cr *CopilotRelay) ensureAPIToken(ctx context.Context, accessToken string) (string, string, error) {
	cr.tokenMu.RLock()
	if cached, ok := cr.tokenCache[accessToken]; ok && cached.expiresAt.After(time.Now().Add(copilotRelayTokenBuffer)) {
		cr.tokenMu.RUnlock()
		return cached.token, cached.apiEndpoint, nil
	}
	cr.tokenMu.RUnlock()

	result, err := sdkcliproxy.ExchangeCopilotAPIToken(ctx, nil, accessToken)
	if err != nil {
		return "", "", fmt.Errorf("copilot token exchange: %w", err)
	}

	apiEndpoint := result.APIEndpoint
	if apiEndpoint == "" {
		apiEndpoint = copilotRelayBaseURL
	}
	apiEndpoint = strings.TrimRight(apiEndpoint, "/")

	expiresAt := time.Now().Add(copilotRelayTokenCacheTTL)
	if result.ExpiresAt > 0 {
		expiresAt = time.Unix(result.ExpiresAt, 0)
	}
	cr.tokenMu.Lock()
	cr.tokenCache[accessToken] = &copilotCachedToken{
		token:       result.Token,
		apiEndpoint: apiEndpoint,
		expiresAt:   expiresAt,
	}
	cr.tokenMu.Unlock()

	return result.Token, apiEndpoint, nil
}

// ProxyNonStreaming executes a non-streaming Copilot API request.
func (cr *CopilotRelay) ProxyNonStreaming(
	ctx context.Context,
	accessToken string,
	model string,
	body map[string]any,
	channelType types.OutboundType,
) (*relay.ProxyResult, error) {
	apiToken, baseURL, err := cr.ensureAPIToken(ctx, accessToken)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}

	model = normalizeRuntimeTargetModel(channelType, model)
	outBody := copyCopilotBody(body)
	outBody["model"] = model
	outBody["stream"] = false
	bodyJSON, _ := json.Marshal(outBody)

	// Determine endpoint: /chat/completions or /responses
	useResponses := shouldUseCopilotResponsesEndpoint(outBody, model)

	// Body normalization pipeline
	bodyJSON = flattenCopilotAssistantContent(bodyJSON)
	bodyJSON = stripCopilotUnsupportedBetas(bodyJSON)
	if useResponses {
		bodyJSON = normalizeCopilotResponsesInput(bodyJSON)
		bodyJSON = normalizeCopilotResponsesTools(bodyJSON)
		bodyJSON = applyCopilotResponsesDefaults(bodyJSON)
	} else {
		bodyJSON = normalizeCopilotChatTools(bodyJSON)
	}

	// Detect vision content before sending
	hasVision := detectCopilotVisionContent(bodyJSON)

	path := copilotChatPath
	if useResponses {
		path = copilotResponsesPath
	}
	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCopilotHeaders(req, apiToken, bodyJSON)
	if hasVision {
		req.Header.Set("Copilot-Vision-Request", "true")
	}

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Do(req)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: http.StatusBadGateway}
	}
	defer func() { _ = resp.Body.Close() }()

	respBytes, err := io.ReadAll(io.LimitReader(resp.Body, 50*1024*1024))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("read response: %v", err), StatusCode: http.StatusBadGateway}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &relay.ProxyError{
			Message:    fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, string(respBytes)),
			StatusCode: resp.StatusCode,
			Headers:    resp.Header.Clone(),
		}
	}

	// Normalize reasoning_text → reasoning_content
	if !useResponses {
		respBytes = normalizeCopilotReasoningField(respBytes)
	}

	var data map[string]any
	if err := json.Unmarshal(respBytes, &data); err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("parse response: %v", err), StatusCode: http.StatusBadGateway}
	}

	usage, _ := data["usage"].(map[string]any)
	return &relay.ProxyResult{
		Response:        data,
		InputTokens:     toIntVal(usage["prompt_tokens"]),
		OutputTokens:    toIntVal(usage["completion_tokens"]),
		StatusCode:      resp.StatusCode,
		UpstreamHeaders: resp.Header.Clone(),
	}, nil
}

// ProxyStreaming executes a streaming Copilot API request.
func (cr *CopilotRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	accessToken string,
	model string,
	body map[string]any,
	channelType types.OutboundType,
	anthropicInbound bool,
	onContent relay.StreamContentCallback,
) (*relay.StreamCompleteInfo, error) {
	apiToken, baseURL, err := cr.ensureAPIToken(ctx, accessToken)
	if err != nil {
		return nil, &relay.ProxyError{Message: err.Error(), StatusCode: http.StatusUnauthorized}
	}

	model = normalizeRuntimeTargetModel(channelType, model)
	outBody := copyCopilotBody(body)
	outBody["model"] = model
	outBody["stream"] = true
	bodyJSON, _ := json.Marshal(outBody)

	// Determine endpoint: /chat/completions or /responses
	useResponses := shouldUseCopilotResponsesEndpoint(outBody, model)

	// Body normalization pipeline
	bodyJSON = flattenCopilotAssistantContent(bodyJSON)
	bodyJSON = stripCopilotUnsupportedBetas(bodyJSON)
	if useResponses {
		bodyJSON = normalizeCopilotResponsesInput(bodyJSON)
		bodyJSON = normalizeCopilotResponsesTools(bodyJSON)
		bodyJSON = applyCopilotResponsesDefaults(bodyJSON)
	} else {
		bodyJSON = normalizeCopilotChatTools(bodyJSON)
		// Enable stream_options for usage stats in chat completions stream
		bodyJSON, _ = sjson.SetBytes(bodyJSON, "stream_options.include_usage", true)
	}

	// Detect vision content before sending
	hasVision := detectCopilotVisionContent(bodyJSON)

	path := copilotChatPath
	if useResponses {
		path = copilotResponsesPath
	}
	url := baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCopilotHeaders(req, apiToken, bodyJSON)
	if hasVision {
		req.Header.Set("Copilot-Vision-Request", "true")
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("upstream request failed: %v", err), StatusCode: http.StatusBadGateway}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		respBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))
		return nil, &relay.ProxyError{
			Message:    fmt.Sprintf("Upstream error %d: %s", resp.StatusCode, string(respBytes)),
			StatusCode: resp.StatusCode,
			Headers:    resp.Header.Clone(),
		}
	}

	// Stream SSE lines to the client.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(nil, copilotMaxScannerBufferSize)

	// Select the appropriate SSE conversion strategy.
	var convertToAnthropic func(string) []string
	var convertResponsesToClaude func([]byte) [][]byte
	if useResponses && anthropicInbound {
		var responsesParam any
		convertResponsesToClaude = func(line []byte) [][]byte {
			return translateCopilotResponsesStreamToClaude(line, &responsesParam)
		}
	} else if anthropicInbound {
		convertToAnthropic = relay.CreateOpenAIToAnthropicSSEConverter()
	}

	var inputTokens, outputTokens int
	var firstTokenTime int
	started := time.Now()
	firstTokenSent := false
	var accContent, accThinking string

	for scanner.Scan() {
		line := scanner.Bytes()

		// --- Write to client ---
		if convertResponsesToClaude != nil {
			// Responses API → Claude format translation
			chunks := convertResponsesToClaude(bytes.Clone(line))
			for _, chunk := range chunks {
				_, _ = w.Write(chunk)
				if flusher != nil {
					flusher.Flush()
				}
			}
		} else if convertToAnthropic != nil {
			// Chat Completions → Anthropic format: convert data: lines.
			if bytes.HasPrefix(line, dataPrefix) {
				// Normalize reasoning_text before conversion
				normalizedLine := normalizeCopilotReasoningFieldSSE(line)
				data := string(normalizedLine[len(dataPrefix):])
				converted := convertToAnthropic(data)
				for _, l := range converted {
					fmt.Fprintf(w, "%s\n", l)
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
		} else {
			// OpenAI passthrough mode: normalize reasoning and write raw lines.
			if bytes.HasPrefix(line, dataPrefix) {
				normalizedLine := normalizeCopilotReasoningFieldSSE(line)
				_, _ = w.Write(normalizedLine)
			} else {
				_, _ = w.Write(line)
			}
			_, _ = w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}

		// --- Track first token time ---
		if !firstTokenSent && bytes.HasPrefix(line, dataPrefix) {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// --- Extract usage and content from original data ---
		if bytes.HasPrefix(line, dataPrefix) {
			chunk := line[len(dataPrefix):]
			if !bytes.Equal(chunk, []byte("[DONE]")) {
				var obj map[string]any
				if json.Unmarshal(chunk, &obj) == nil {
					if usage, ok := obj["usage"].(map[string]any); ok {
						inputTokens = toIntVal(usage["prompt_tokens"])
						outputTokens = toIntVal(usage["completion_tokens"])
					}
					// Accumulate content and thinking for callback.
					if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]any); ok {
							if delta, ok := choice["delta"].(map[string]any); ok {
								if c, ok := delta["content"].(string); ok {
									accContent += c
								}
								// Accumulate reasoning_content (normalized from reasoning_text)
								if rc, ok := delta["reasoning_content"].(string); ok {
									accThinking += rc
								}
							}
						}
					}
				}
			}
		}
	}
	_ = resp.Body.Close()

	if onContent != nil {
		onContent(accThinking, accContent)
	}

	return &relay.StreamCompleteInfo{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		FirstTokenTime:  firstTokenTime,
		ResponseContent: accContent,
		ThinkingContent: accThinking,
		UpstreamHeaders: resp.Header.Clone(),
	}, nil
}

// applyCopilotHeaders sets the required Copilot-specific headers on a request.
func applyCopilotHeaders(r *http.Request, apiToken string, body []byte) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+apiToken)
	r.Header.Set("Accept", "application/json")
	r.Header.Set("User-Agent", copilotRelayUserAgent)
	r.Header.Set("Editor-Version", copilotRelayEditorVersion)
	r.Header.Set("Editor-Plugin-Version", copilotRelayPluginVersion)
	r.Header.Set("Openai-Intent", copilotRelayOpenAIIntent)
	r.Header.Set("Copilot-Integration-Id", copilotRelayIntegrationID)
	r.Header.Set("X-Github-Api-Version", copilotRelayGitHubAPIVer)
	r.Header.Set("X-Request-Id", uuid.NewString())

	initiator := "user"
	if isCopilotAgentInitiated(body) {
		initiator = "agent"
	}
	r.Header.Set("X-Initiator", initiator)
}

// toIntVal converts an any to int (handles float64 from JSON).
func toIntVal(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	default:
		return 0
	}
}

// copyCopilotBody makes a shallow copy of a JSON body map.
func copyCopilotBody(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	return out
}

// convertAnthropicBodyToOpenAI translates an Anthropic Messages request body
// into OpenAI Chat Completions format so that Copilot /chat/completions can
// consume it.  The conversion covers messages, system, tools, and tool_choice.
func convertAnthropicBodyToOpenAI(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))

	// --- messages -----------------------------------------------------------
	var openAIMessages []any

	// Hoist top-level "system" into a system message.
	if sys := body["system"]; sys != nil {
		sysStr := ""
		switch v := sys.(type) {
		case string:
			sysStr = v
		case []any:
			// Anthropic system can be an array of content blocks.
			for _, blk := range v {
				if m, ok := blk.(map[string]any); ok {
					if t, ok := m["text"].(string); ok {
						if sysStr != "" {
							sysStr += "\n"
						}
						sysStr += t
					}
				}
			}
		default:
			b, _ := json.Marshal(sys)
			sysStr = string(b)
		}
		if sysStr != "" {
			openAIMessages = append(openAIMessages, map[string]any{
				"role":    "system",
				"content": sysStr,
			})
		}
	}

	if msgs, ok := body["messages"].([]any); ok {
		for _, m := range msgs {
			msg, ok := m.(map[string]any)
			if !ok {
				continue
			}
			role, _ := msg["role"].(string)
			switch role {
			case "assistant":
				openAIMessages = append(openAIMessages, convertAnthropicAssistantToOpenAI(msg))
			case "user":
				// convertAnthropicUserToOpenAI may return a single message or a []any slice
				// (when tool_result blocks are mixed with text). Flatten into the result.
				userResult := convertAnthropicUserToOpenAI(msg)
				if slice, ok := userResult.([]any); ok {
					openAIMessages = append(openAIMessages, slice...)
				} else {
					openAIMessages = append(openAIMessages, userResult)
				}
			default:
				// Pass through as-is (e.g. "system" inside messages).
				openAIMessages = append(openAIMessages, msg)
			}
		}
	}
	out["messages"] = openAIMessages

	// --- tools --------------------------------------------------------------
	if tools, ok := body["tools"].([]any); ok {
		out["tools"] = convertAnthropicToolsToOpenAI(tools)
	}

	// --- tool_choice --------------------------------------------------------
	if tc := body["tool_choice"]; tc != nil {
		out["tool_choice"] = normalizeToolChoiceForOpenAI(tc)
	}

	// --- scalar fields that are directly compatible -------------------------
	for _, key := range []string{
		"model", "stream", "temperature", "top_p", "max_tokens",
		"top_k", "metadata",
	} {
		if v, ok := body[key]; ok {
			out[key] = v
		}
	}

	// Map Anthropic "stop_sequences" → OpenAI "stop".
	if ss, ok := body["stop_sequences"]; ok {
		out["stop"] = ss
	}

	return out
}

// convertAnthropicAssistantToOpenAI converts an Anthropic assistant message
// (possibly with tool_use content blocks) to an OpenAI assistant message.
func convertAnthropicAssistantToOpenAI(msg map[string]any) map[string]any {
	content := msg["content"]

	// Simple string content — pass through.
	if s, ok := content.(string); ok {
		return map[string]any{"role": "assistant", "content": s}
	}

	// Content blocks.
	blocks, ok := content.([]any)
	if !ok || len(blocks) == 0 {
		return map[string]any{"role": "assistant", "content": ""}
	}

	var textParts []string
	var toolCalls []any
	tcIdx := 0
	for _, b := range blocks {
		blk, ok := b.(map[string]any)
		if !ok {
			continue
		}
		bType, _ := blk["type"].(string)
		switch bType {
		case "text":
			if t, ok := blk["text"].(string); ok {
				textParts = append(textParts, t)
			}
		case "tool_use":
			id, _ := blk["id"].(string)
			if id == "" {
				id = fmt.Sprintf("call_%d", tcIdx)
			}
			name, _ := blk["name"].(string)
			inputJSON, _ := json.Marshal(blk["input"])
			toolCalls = append(toolCalls, map[string]any{
				"id":   id,
				"type": "function",
				"function": map[string]any{
					"name":      name,
					"arguments": string(inputJSON),
				},
			})
			tcIdx++
		}
	}

	result := map[string]any{"role": "assistant"}
	text := strings.Join(textParts, "")
	if text != "" {
		result["content"] = text
	} else {
		result["content"] = nil
	}
	if len(toolCalls) > 0 {
		result["tool_calls"] = toolCalls
	}
	return result
}

// convertAnthropicUserToOpenAI converts an Anthropic user message to OpenAI format.
// Handles both simple text and content blocks (including tool_result).
func convertAnthropicUserToOpenAI(msg map[string]any) any {
	content := msg["content"]

	// Simple string content.
	if s, ok := content.(string); ok {
		return map[string]any{"role": "user", "content": s}
	}

	blocks, ok := content.([]any)
	if !ok || len(blocks) == 0 {
		return map[string]any{"role": "user", "content": ""}
	}

	// Check for tool_result blocks — each becomes a separate "tool" role message.
	// If mixed with text, we split them out.
	var result []any
	var textParts []string

	for _, b := range blocks {
		blk, ok := b.(map[string]any)
		if !ok {
			continue
		}
		bType, _ := blk["type"].(string)
		switch bType {
		case "tool_result":
			// Flush pending text.
			if len(textParts) > 0 {
				result = append(result, map[string]any{
					"role":    "user",
					"content": strings.Join(textParts, ""),
				})
				textParts = nil
			}
			toolUseID, _ := blk["tool_use_id"].(string)
			var contentStr string
			switch c := blk["content"].(type) {
			case string:
				contentStr = c
			case []any:
				// Extract text from content blocks.
				for _, cb := range c {
					if cm, ok := cb.(map[string]any); ok {
						if t, ok := cm["text"].(string); ok {
							contentStr += t
						}
					}
				}
			default:
				if blk["content"] != nil {
					b, _ := json.Marshal(blk["content"])
					contentStr = string(b)
				}
			}
			result = append(result, map[string]any{
				"role":         "tool",
				"tool_call_id": toolUseID,
				"content":      contentStr,
			})
		case "text":
			if t, ok := blk["text"].(string); ok {
				textParts = append(textParts, t)
			}
		case "image":
			// Pass image content through as OpenAI image_url format.
			if src, ok := blk["source"].(map[string]any); ok {
				mediaType, _ := src["media_type"].(string)
				data, _ := src["data"].(string)
				result = append(result, map[string]any{
					"role": "user",
					"content": []any{
						map[string]any{
							"type": "image_url",
							"image_url": map[string]any{
								"url": "data:" + mediaType + ";base64," + data,
							},
						},
					},
				})
			}
		}
	}

	// Flush remaining text.
	if len(textParts) > 0 {
		result = append(result, map[string]any{
			"role":    "user",
			"content": strings.Join(textParts, ""),
		})
	}

	if len(result) == 1 {
		return result[0]
	}
	return result
}

// convertAnthropicToolsToOpenAI converts Anthropic tools (with input_schema)
// to OpenAI function-calling tools format.
func convertAnthropicToolsToOpenAI(tools []any) []any {
	var result []any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		// Already in OpenAI format (has "function" key) — pass through.
		if _, hasFn := tool["function"]; hasFn {
			result = append(result, tool)
			continue
		}
		name, _ := tool["name"].(string)
		desc, _ := tool["description"].(string)
		params := tool["input_schema"]
		if params == nil {
			params = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		result = append(result, map[string]any{
			"type": "function",
			"function": map[string]any{
				"name":        name,
				"description": desc,
				"parameters":  params,
			},
		})
	}
	return result
}

// normalizeToolChoiceForOpenAI converts Anthropic tool_choice format to OpenAI format.
//
//	Anthropic {"type": "auto"}    → OpenAI "auto"
//	Anthropic {"type": "any"}     → OpenAI "required"
//	Anthropic {"type": "none"}    → OpenAI "none"
//	Anthropic {"type": "tool", "name": "X"} → OpenAI {"type": "function", "function": {"name": "X"}}
//	string values pass through as-is.
func normalizeToolChoiceForOpenAI(tc any) any {
	// Already a string (OpenAI format) — pass through.
	if s, ok := tc.(string); ok {
		return s
	}

	tcMap, ok := tc.(map[string]any)
	if !ok {
		return "auto"
	}

	tcType, _ := tcMap["type"].(string)
	switch tcType {
	case "auto":
		return "auto"
	case "any":
		return "required"
	case "none":
		return "none"
	case "tool":
		name, _ := tcMap["name"].(string)
		if name == "" {
			return "auto"
		}
		return map[string]any{
			"type": "function",
			"function": map[string]any{
				"name": name,
			},
		}
	default:
		// Unknown type — check if it looks like an OpenAI function call object.
		if tcType == "function" {
			return tcMap
		}
		return "auto"
	}
}

// ---------------------------------------------------------------------------
// Copilot-specific request/response normalization functions
// Ported from CLIProxyAPIPlus github_copilot_executor.go
// ---------------------------------------------------------------------------

// shouldUseCopilotResponsesEndpoint determines whether to use /responses
// instead of /chat/completions.
func shouldUseCopilotResponsesEndpoint(body map[string]any, model string) bool {
	if _, hasInput := body["input"]; hasInput {
		return true
	}
	return strings.Contains(strings.ToLower(model), "codex")
}

// isCopilotAgentInitiated determines whether the current request is
// agent-initiated (tool callbacks, continuations) rather than user-initiated.
//
// GitHub Copilot uses the X-Initiator header for billing:
//   - "user"  → consumes premium request quota
//   - "agent" → free (tool loops, continuations)
func isCopilotAgentInitiated(body []byte) bool {
	if len(body) == 0 {
		return false
	}

	// Chat Completions API: check messages array
	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		arr := messages.Array()
		if len(arr) == 0 {
			return false
		}

		lastRole := ""
		for i := len(arr) - 1; i >= 0; i-- {
			if r := arr[i].Get("role").String(); r != "" {
				lastRole = r
				break
			}
		}

		if lastRole == "assistant" || lastRole == "tool" {
			return true
		}

		if lastRole == "user" {
			lastContent := arr[len(arr)-1].Get("content")
			if lastContent.Exists() && lastContent.IsArray() {
				for _, part := range lastContent.Array() {
					if part.Get("type").String() == "tool_result" {
						return true
					}
				}
			}
			if len(arr) >= 2 {
				prev := arr[len(arr)-2]
				if prev.Get("role").String() == "assistant" {
					prevContent := prev.Get("content")
					if prevContent.Exists() && prevContent.IsArray() {
						for _, part := range prevContent.Array() {
							if part.Get("type").String() == "tool_use" {
								return true
							}
						}
					}
				}
			}
		}
		return false
	}

	// Responses API: check input array
	if inputs := gjson.GetBytes(body, "input"); inputs.Exists() && inputs.IsArray() {
		arr := inputs.Array()
		if len(arr) == 0 {
			return false
		}
		last := arr[len(arr)-1]
		if role := last.Get("role").String(); role == "assistant" {
			return true
		}
		switch last.Get("type").String() {
		case "function_call", "function_call_arguments", "computer_call":
			return true
		case "function_call_output", "function_call_response", "tool_result", "computer_call_output":
			return true
		}
		for _, item := range arr {
			if role := item.Get("role").String(); role == "assistant" {
				return true
			}
			switch item.Get("type").String() {
			case "function_call", "function_call_output", "function_call_response",
				"function_call_arguments", "computer_call", "computer_call_output":
				return true
			}
		}
	}
	return false
}

// detectCopilotVisionContent checks if the request body contains image content.
func detectCopilotVisionContent(body []byte) bool {
	messagesResult := gjson.GetBytes(body, "messages")
	if !messagesResult.Exists() || !messagesResult.IsArray() {
		return false
	}
	for _, message := range messagesResult.Array() {
		content := message.Get("content")
		if content.IsArray() {
			for _, block := range content.Array() {
				blockType := block.Get("type").String()
				if blockType == "image_url" || blockType == "image" {
					return true
				}
			}
		}
	}
	return false
}

// flattenCopilotAssistantContent converts assistant message content from array
// format to a joined string. GitHub Copilot requires assistant content as a
// string; sending it as an array causes Claude models to re-answer all
// previous prompts.
func flattenCopilotAssistantContent(body []byte) []byte {
	messages := gjson.GetBytes(body, "messages")
	if !messages.Exists() || !messages.IsArray() {
		return body
	}
	result := body
	for i, msg := range messages.Array() {
		if msg.Get("role").String() != "assistant" {
			continue
		}
		content := msg.Get("content")
		if !content.Exists() || !content.IsArray() {
			continue
		}
		hasNonText := false
		var textParts []string
		for _, part := range content.Array() {
			if t := part.Get("type").String(); t != "" && t != "text" {
				hasNonText = true
				break
			}
			if part.Get("type").String() == "text" {
				if txt := part.Get("text").String(); txt != "" {
					textParts = append(textParts, txt)
				}
			}
		}
		if hasNonText {
			continue
		}
		result, _ = sjson.SetBytes(result, fmt.Sprintf("messages.%d.content", i), strings.Join(textParts, ""))
	}
	return result
}

// normalizeCopilotChatTools filters tools to only type:"function" and
// normalizes invalid tool_choice values to "auto".
func normalizeCopilotChatTools(body []byte) []byte {
	tools := gjson.GetBytes(body, "tools")
	if tools.Exists() {
		filtered := "[]"
		if tools.IsArray() {
			for _, tool := range tools.Array() {
				if tool.Get("type").String() != "function" {
					continue
				}
				filtered, _ = sjson.SetRaw(filtered, "-1", tool.Raw)
			}
		}
		body, _ = sjson.SetRawBytes(body, "tools", []byte(filtered))
	}

	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return body
	}
	if toolChoice.Type == gjson.String {
		switch toolChoice.String() {
		case "auto", "none", "required":
			return body
		}
	}
	body, _ = sjson.SetBytes(body, "tool_choice", "auto")
	return body
}

// stripCopilotUnsupportedBetas removes Anthropic-specific beta entries.
func stripCopilotUnsupportedBetas(body []byte) []byte {
	betaPaths := []string{"betas", "metadata.betas"}
	for _, path := range betaPaths {
		arr := gjson.GetBytes(body, path)
		if !arr.Exists() || !arr.IsArray() {
			continue
		}
		var filtered []string
		changed := false
		for _, item := range arr.Array() {
			beta := item.String()
			if isCopilotUnsupportedBeta(beta) {
				changed = true
				continue
			}
			filtered = append(filtered, beta)
		}
		if !changed {
			continue
		}
		if len(filtered) == 0 {
			body, _ = sjson.DeleteBytes(body, path)
		} else {
			body, _ = sjson.SetBytes(body, path, filtered)
		}
	}
	return body
}

func isCopilotUnsupportedBeta(beta string) bool {
	return slices.Contains(copilotUnsupportedBetas, beta)
}

// normalizeCopilotReasoningField maps Copilot's non-standard 'reasoning_text'
// field to the standard OpenAI 'reasoning_content' field.
func normalizeCopilotReasoningField(data []byte) []byte {
	choices := gjson.GetBytes(data, "choices")
	if !choices.Exists() || !choices.IsArray() {
		return data
	}
	for i := range choices.Array() {
		msgRT := fmt.Sprintf("choices.%d.message.reasoning_text", i)
		msgRC := fmt.Sprintf("choices.%d.message.reasoning_content", i)
		if rt := gjson.GetBytes(data, msgRT); rt.Exists() && rt.String() != "" {
			if rc := gjson.GetBytes(data, msgRC); !rc.Exists() || rc.Type == gjson.Null || rc.String() == "" {
				data, _ = sjson.SetBytes(data, msgRC, rt.String())
			}
		}
		deltaRT := fmt.Sprintf("choices.%d.delta.reasoning_text", i)
		deltaRC := fmt.Sprintf("choices.%d.delta.reasoning_content", i)
		if rt := gjson.GetBytes(data, deltaRT); rt.Exists() && rt.String() != "" {
			if rc := gjson.GetBytes(data, deltaRC); !rc.Exists() || rc.Type == gjson.Null || rc.String() == "" {
				data, _ = sjson.SetBytes(data, deltaRC, rt.String())
			}
		}
	}
	return data
}

// normalizeCopilotReasoningFieldSSE applies reasoning_text normalization to
// an SSE data: line, returning the normalized line.
func normalizeCopilotReasoningFieldSSE(line []byte) []byte {
	if !bytes.HasPrefix(line, dataPrefix) {
		return line
	}
	sseData := bytes.TrimSpace(line[len(dataPrefix):])
	if bytes.Equal(sseData, []byte("[DONE]")) || !gjson.ValidBytes(sseData) {
		return line
	}
	normalized := normalizeCopilotReasoningField(bytes.Clone(sseData))
	if bytes.Equal(normalized, sseData) {
		return line
	}
	return append(append([]byte(nil), dataPrefix...), normalized...)
}

// ---------------------------------------------------------------------------
// Copilot Responses API normalization
// ---------------------------------------------------------------------------

// normalizeCopilotResponsesInput converts Claude messages format to OpenAI
// Responses API input array.
func normalizeCopilotResponsesInput(body []byte) []byte {
	body = stripCopilotResponsesUnsupportedFields(body)
	input := gjson.GetBytes(body, "input")
	if input.Exists() {
		if input.Type == gjson.String || input.IsArray() {
			return body
		}
		body, _ = sjson.SetBytes(body, "input", input.Raw)
		return body
	}

	inputArr := "[]"

	// System messages → developer role
	if system := gjson.GetBytes(body, "system"); system.Exists() {
		var systemParts []string
		if system.IsArray() {
			for _, part := range system.Array() {
				if txt := part.Get("text").String(); txt != "" {
					systemParts = append(systemParts, txt)
				}
			}
		} else if system.Type == gjson.String {
			systemParts = append(systemParts, system.String())
		}
		if len(systemParts) > 0 {
			msg := `{"type":"message","role":"developer","content":[]}`
			for _, txt := range systemParts {
				part := `{"type":"input_text","text":""}`
				part, _ = sjson.Set(part, "text", txt)
				msg, _ = sjson.SetRaw(msg, "content.-1", part)
			}
			inputArr, _ = sjson.SetRaw(inputArr, "-1", msg)
		}
	}

	// Messages → structured input items
	if messages := gjson.GetBytes(body, "messages"); messages.Exists() && messages.IsArray() {
		for _, msg := range messages.Array() {
			role := msg.Get("role").String()
			content := msg.Get("content")
			if !content.Exists() {
				continue
			}
			if content.Type == gjson.String {
				textType := "input_text"
				if role == "assistant" {
					textType = "output_text"
				}
				item := `{"type":"message","role":"","content":[]}`
				item, _ = sjson.Set(item, "role", role)
				part := fmt.Sprintf(`{"type":"%s","text":""}`, textType)
				part, _ = sjson.Set(part, "text", content.String())
				item, _ = sjson.SetRaw(item, "content.-1", part)
				inputArr, _ = sjson.SetRaw(inputArr, "-1", item)
				continue
			}
			if !content.IsArray() {
				continue
			}
			var msgParts []string
			for _, c := range content.Array() {
				cType := c.Get("type").String()
				switch cType {
				case "text":
					textType := "input_text"
					if role == "assistant" {
						textType = "output_text"
					}
					part := fmt.Sprintf(`{"type":"%s","text":""}`, textType)
					part, _ = sjson.Set(part, "text", c.Get("text").String())
					msgParts = append(msgParts, part)
				case "image":
					source := c.Get("source")
					if source.Exists() {
						data := source.Get("data").String()
						if data == "" {
							data = source.Get("base64").String()
						}
						mediaType := source.Get("media_type").String()
						if mediaType == "" {
							mediaType = source.Get("mime_type").String()
						}
						if mediaType == "" {
							mediaType = "application/octet-stream"
						}
						if data != "" {
							part := `{"type":"input_image","image_url":""}`
							part, _ = sjson.Set(part, "image_url", fmt.Sprintf("data:%s;base64,%s", mediaType, data))
							msgParts = append(msgParts, part)
						}
					}
				case "tool_use":
					if len(msgParts) > 0 {
						item := `{"type":"message","role":"","content":[]}`
						item, _ = sjson.Set(item, "role", role)
						for _, p := range msgParts {
							item, _ = sjson.SetRaw(item, "content.-1", p)
						}
						inputArr, _ = sjson.SetRaw(inputArr, "-1", item)
						msgParts = nil
					}
					fc := `{"type":"function_call","call_id":"","name":"","arguments":""}`
					fc, _ = sjson.Set(fc, "call_id", c.Get("id").String())
					fc, _ = sjson.Set(fc, "name", c.Get("name").String())
					if inputRaw := c.Get("input"); inputRaw.Exists() {
						fc, _ = sjson.Set(fc, "arguments", inputRaw.Raw)
					}
					inputArr, _ = sjson.SetRaw(inputArr, "-1", fc)
				case "tool_result":
					if len(msgParts) > 0 {
						item := `{"type":"message","role":"","content":[]}`
						item, _ = sjson.Set(item, "role", role)
						for _, p := range msgParts {
							item, _ = sjson.SetRaw(item, "content.-1", p)
						}
						inputArr, _ = sjson.SetRaw(inputArr, "-1", item)
						msgParts = nil
					}
					fco := `{"type":"function_call_output","call_id":"","output":""}`
					fco, _ = sjson.Set(fco, "call_id", c.Get("tool_use_id").String())
					resultContent := c.Get("content")
					if resultContent.Type == gjson.String {
						fco, _ = sjson.Set(fco, "output", resultContent.String())
					} else if resultContent.IsArray() {
						var resultParts []string
						for _, rc := range resultContent.Array() {
							if txt := rc.Get("text").String(); txt != "" {
								resultParts = append(resultParts, txt)
							}
						}
						fco, _ = sjson.Set(fco, "output", strings.Join(resultParts, "\n"))
					} else if resultContent.Exists() {
						fco, _ = sjson.Set(fco, "output", resultContent.String())
					}
					inputArr, _ = sjson.SetRaw(inputArr, "-1", fco)
				case "thinking":
					// Skip thinking blocks
				}
			}
			if len(msgParts) > 0 {
				item := `{"type":"message","role":"","content":[]}`
				item, _ = sjson.Set(item, "role", role)
				for _, p := range msgParts {
					item, _ = sjson.SetRaw(item, "content.-1", p)
				}
				inputArr, _ = sjson.SetRaw(inputArr, "-1", item)
			}
		}
	}

	body, _ = sjson.SetRawBytes(body, "input", []byte(inputArr))
	body, _ = sjson.DeleteBytes(body, "messages")
	body, _ = sjson.DeleteBytes(body, "system")
	return body
}

func stripCopilotResponsesUnsupportedFields(body []byte) []byte {
	body, _ = sjson.DeleteBytes(body, "service_tier")
	return body
}

// normalizeCopilotResponsesTools standardizes tools for the Responses API.
func normalizeCopilotResponsesTools(body []byte) []byte {
	tools := gjson.GetBytes(body, "tools")
	if tools.Exists() {
		filtered := "[]"
		if tools.IsArray() {
			for _, tool := range tools.Array() {
				toolType := tool.Get("type").String()
				if toolType == "computer" || toolType == "computer_use_preview" {
					filtered, _ = sjson.SetRaw(filtered, "-1", tool.Raw)
					continue
				}
				if toolType != "" && toolType != "function" {
					continue
				}
				name := tool.Get("name").String()
				if name == "" {
					name = tool.Get("function.name").String()
				}
				if name == "" {
					continue
				}
				normalized := `{"type":"function","name":""}`
				normalized, _ = sjson.Set(normalized, "name", name)
				if desc := tool.Get("description").String(); desc != "" {
					normalized, _ = sjson.Set(normalized, "description", desc)
				} else if desc = tool.Get("function.description").String(); desc != "" {
					normalized, _ = sjson.Set(normalized, "description", desc)
				}
				if params := tool.Get("parameters"); params.Exists() {
					normalized, _ = sjson.SetRaw(normalized, "parameters", params.Raw)
				} else if params = tool.Get("function.parameters"); params.Exists() {
					normalized, _ = sjson.SetRaw(normalized, "parameters", params.Raw)
				} else if params = tool.Get("input_schema"); params.Exists() {
					normalized, _ = sjson.SetRaw(normalized, "parameters", params.Raw)
				}
				filtered, _ = sjson.SetRaw(filtered, "-1", normalized)
			}
		}
		body, _ = sjson.SetRawBytes(body, "tools", []byte(filtered))
	}

	toolChoice := gjson.GetBytes(body, "tool_choice")
	if !toolChoice.Exists() {
		return body
	}
	if toolChoice.Type == gjson.String {
		switch toolChoice.String() {
		case "auto", "none", "required":
			return body
		default:
			body, _ = sjson.SetBytes(body, "tool_choice", "auto")
			return body
		}
	}
	if toolChoice.Type == gjson.JSON {
		choiceType := toolChoice.Get("type").String()
		if choiceType == "computer" || choiceType == "computer_use_preview" {
			return body
		}
		if choiceType == "function" {
			name := toolChoice.Get("name").String()
			if name == "" {
				name = toolChoice.Get("function.name").String()
			}
			if name != "" {
				normalized := `{"type":"function","name":""}`
				normalized, _ = sjson.Set(normalized, "name", name)
				body, _ = sjson.SetRawBytes(body, "tool_choice", []byte(normalized))
				return body
			}
		}
	}
	body, _ = sjson.SetBytes(body, "tool_choice", "auto")
	return body
}

// applyCopilotResponsesDefaults sets required fields for the Responses API.
func applyCopilotResponsesDefaults(body []byte) []byte {
	if !gjson.GetBytes(body, "store").Exists() {
		body, _ = sjson.SetBytes(body, "store", false)
	}
	if !gjson.GetBytes(body, "include").Exists() {
		body, _ = sjson.SetRawBytes(body, "include", []byte(`["reasoning.encrypted_content"]`))
	}
	if gjson.GetBytes(body, "reasoning.effort").Exists() && !gjson.GetBytes(body, "reasoning.summary").Exists() {
		body, _ = sjson.SetBytes(body, "reasoning.summary", "auto")
	}
	return body
}

// ---------------------------------------------------------------------------
// Copilot Responses API → Claude format translation
// ---------------------------------------------------------------------------

type copilotResponsesStreamToolState struct {
	Index int
	ID    string
	Name  string
}

type copilotResponsesStreamState struct {
	MessageStarted    bool
	MessageStopSent   bool
	TextBlockStarted  bool
	TextBlockIndex    int
	NextContentIndex  int
	HasToolUse        bool
	ReasoningActive   bool
	ReasoningIndex    int
	OutputIndexToTool map[int]*copilotResponsesStreamToolState
	ItemIDToTool      map[string]*copilotResponsesStreamToolState
}

func translateCopilotResponsesStreamToClaude(line []byte, param *any) [][]byte {
	if *param == nil {
		*param = &copilotResponsesStreamState{
			TextBlockIndex:    -1,
			OutputIndexToTool: make(map[int]*copilotResponsesStreamToolState),
			ItemIDToTool:      make(map[string]*copilotResponsesStreamToolState),
		}
	}
	state := (*param).(*copilotResponsesStreamState)

	if !bytes.HasPrefix(line, dataPrefix) {
		return nil
	}
	payload := bytes.TrimSpace(line[len(dataPrefix):])
	if bytes.Equal(payload, []byte("[DONE]")) {
		return nil
	}
	if !gjson.ValidBytes(payload) {
		return nil
	}

	event := gjson.GetBytes(payload, "type").String()
	results := make([][]byte, 0, 4)
	appendResult := func(chunk string) {
		results = append(results, []byte(chunk))
	}
	ensureMessageStart := func() {
		if state.MessageStarted {
			return
		}
		ms := `{"type":"message_start","message":{"id":"","type":"message","role":"assistant","model":"","content":[],"stop_reason":null,"stop_sequence":null,"usage":{"input_tokens":0,"output_tokens":0}}}`
		ms, _ = sjson.Set(ms, "message.id", gjson.GetBytes(payload, "response.id").String())
		ms, _ = sjson.Set(ms, "message.model", gjson.GetBytes(payload, "response.model").String())
		appendResult("event: message_start\ndata: " + ms + "\n\n")
		state.MessageStarted = true
	}
	startTextBlockIfNeeded := func() {
		if state.TextBlockStarted {
			return
		}
		if state.TextBlockIndex < 0 {
			state.TextBlockIndex = state.NextContentIndex
			state.NextContentIndex++
		}
		cbs := `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`
		cbs, _ = sjson.Set(cbs, "index", state.TextBlockIndex)
		appendResult("event: content_block_start\ndata: " + cbs + "\n\n")
		state.TextBlockStarted = true
	}
	stopTextBlockIfNeeded := func() {
		if !state.TextBlockStarted {
			return
		}
		cbStop := `{"type":"content_block_stop","index":0}`
		cbStop, _ = sjson.Set(cbStop, "index", state.TextBlockIndex)
		appendResult("event: content_block_stop\ndata: " + cbStop + "\n\n")
		state.TextBlockStarted = false
		state.TextBlockIndex = -1
	}
	resolveTool := func(itemID string, outputIndex int) *copilotResponsesStreamToolState {
		if itemID != "" {
			if tool, ok := state.ItemIDToTool[itemID]; ok {
				return tool
			}
		}
		if tool, ok := state.OutputIndexToTool[outputIndex]; ok {
			if itemID != "" {
				state.ItemIDToTool[itemID] = tool
			}
			return tool
		}
		return nil
	}

	switch event {
	case "response.created":
		ensureMessageStart()
	case "response.output_text.delta":
		ensureMessageStart()
		startTextBlockIfNeeded()
		delta := gjson.GetBytes(payload, "delta").String()
		if delta != "" {
			cd := `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":""}}`
			cd, _ = sjson.Set(cd, "index", state.TextBlockIndex)
			cd, _ = sjson.Set(cd, "delta.text", delta)
			appendResult("event: content_block_delta\ndata: " + cd + "\n\n")
		}
	case "response.reasoning_summary_part.added":
		ensureMessageStart()
		state.ReasoningActive = true
		state.ReasoningIndex = state.NextContentIndex
		state.NextContentIndex++
		ts := `{"type":"content_block_start","index":0,"content_block":{"type":"thinking","thinking":""}}`
		ts, _ = sjson.Set(ts, "index", state.ReasoningIndex)
		appendResult("event: content_block_start\ndata: " + ts + "\n\n")
	case "response.reasoning_summary_text.delta":
		if state.ReasoningActive {
			delta := gjson.GetBytes(payload, "delta").String()
			if delta != "" {
				td := `{"type":"content_block_delta","index":0,"delta":{"type":"thinking_delta","thinking":""}}`
				td, _ = sjson.Set(td, "index", state.ReasoningIndex)
				td, _ = sjson.Set(td, "delta.thinking", delta)
				appendResult("event: content_block_delta\ndata: " + td + "\n\n")
			}
		}
	case "response.reasoning_summary_part.done":
		if state.ReasoningActive {
			tStop := `{"type":"content_block_stop","index":0}`
			tStop, _ = sjson.Set(tStop, "index", state.ReasoningIndex)
			appendResult("event: content_block_stop\ndata: " + tStop + "\n\n")
			state.ReasoningActive = false
		}
	case "response.output_item.added":
		if gjson.GetBytes(payload, "item.type").String() != "function_call" {
			break
		}
		ensureMessageStart()
		stopTextBlockIfNeeded()
		state.HasToolUse = true
		tool := &copilotResponsesStreamToolState{
			Index: state.NextContentIndex,
			ID:    gjson.GetBytes(payload, "item.call_id").String(),
			Name:  gjson.GetBytes(payload, "item.name").String(),
		}
		if tool.ID == "" {
			tool.ID = gjson.GetBytes(payload, "item.id").String()
		}
		state.NextContentIndex++
		outputIndex := int(gjson.GetBytes(payload, "output_index").Int())
		state.OutputIndexToTool[outputIndex] = tool
		if itemID := gjson.GetBytes(payload, "item.id").String(); itemID != "" {
			state.ItemIDToTool[itemID] = tool
		}
		cbs := `{"type":"content_block_start","index":0,"content_block":{"type":"tool_use","id":"","name":"","input":{}}}`
		cbs, _ = sjson.Set(cbs, "index", tool.Index)
		cbs, _ = sjson.Set(cbs, "content_block.id", tool.ID)
		cbs, _ = sjson.Set(cbs, "content_block.name", tool.Name)
		appendResult("event: content_block_start\ndata: " + cbs + "\n\n")
	case "response.function_call_arguments.delta":
		itemID := gjson.GetBytes(payload, "item_id").String()
		outputIndex := int(gjson.GetBytes(payload, "output_index").Int())
		tool := resolveTool(itemID, outputIndex)
		if tool == nil {
			break
		}
		partial := gjson.GetBytes(payload, "delta").String()
		if partial == "" {
			break
		}
		id := `{"type":"content_block_delta","index":0,"delta":{"type":"input_json_delta","partial_json":""}}`
		id, _ = sjson.Set(id, "index", tool.Index)
		id, _ = sjson.Set(id, "delta.partial_json", partial)
		appendResult("event: content_block_delta\ndata: " + id + "\n\n")
	case "response.output_item.done":
		if gjson.GetBytes(payload, "item.type").String() != "function_call" {
			break
		}
		tool := resolveTool(gjson.GetBytes(payload, "item.id").String(), int(gjson.GetBytes(payload, "output_index").Int()))
		if tool == nil {
			break
		}
		cbStop := `{"type":"content_block_stop","index":0}`
		cbStop, _ = sjson.Set(cbStop, "index", tool.Index)
		appendResult("event: content_block_stop\ndata: " + cbStop + "\n\n")
	case "response.completed":
		ensureMessageStart()
		stopTextBlockIfNeeded()
		if !state.MessageStopSent {
			stopReason := "end_turn"
			if state.HasToolUse {
				stopReason = "tool_use"
			} else if sr := gjson.GetBytes(payload, "response.stop_reason").String(); sr == "max_tokens" || sr == "stop" {
				stopReason = sr
			}
			inputTokens := gjson.GetBytes(payload, "response.usage.input_tokens").Int()
			outputTokens := gjson.GetBytes(payload, "response.usage.output_tokens").Int()
			cachedTokens := gjson.GetBytes(payload, "response.usage.input_tokens_details.cached_tokens").Int()
			if cachedTokens > 0 && inputTokens >= cachedTokens {
				inputTokens -= cachedTokens
			}
			md := `{"type":"message_delta","delta":{"stop_reason":"","stop_sequence":null},"usage":{"input_tokens":0,"output_tokens":0}}`
			md, _ = sjson.Set(md, "delta.stop_reason", stopReason)
			md, _ = sjson.Set(md, "usage.input_tokens", inputTokens)
			md, _ = sjson.Set(md, "usage.output_tokens", outputTokens)
			if cachedTokens > 0 {
				md, _ = sjson.Set(md, "usage.cache_read_input_tokens", cachedTokens)
			}
			appendResult("event: message_delta\ndata: " + md + "\n\n")
			appendResult("event: message_stop\ndata: {\"type\":\"message_stop\"}\n\n")
			state.MessageStopSent = true
		}
	}

	return results
}

// executeCopilotNonStreaming is called by the non-stream strategy for Copilot channels.
func (h *RelayHandler) executeCopilotNonStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, err := h.CopilotRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve copilot access token: %v", err), StatusCode: http.StatusUnauthorized}
	}

	result, proxyErr := h.CopilotRelay.ProxyNonStreaming(
		p.C.Request.Context(),
		accessToken,
		p.TargetModel,
		p.Body,
		p.Channel.Type,
	)
	if proxyErr != nil {
		return nil, proxyErr
	}

	respJSON, _ := json.Marshal(result.Response)
	return &relayResult{
		InputTokens:     result.InputTokens,
		OutputTokens:    result.OutputTokens,
		Response:        result.Response,
		ResponseContent: string(respJSON),
		ResponseHeaders: result.UpstreamHeaders,
	}, nil
}

// executeCopilotStreaming is called by the stream strategy for Copilot channels.
func (h *RelayHandler) executeCopilotStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, err := h.CopilotRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve copilot access token: %v", err), StatusCode: http.StatusUnauthorized}
	}

	streamId := fmt.Sprintf("%d-%d-%d", time.Now().UnixNano(), p.Channel.ID, p.ApiKeyID)

	h.Observer.StreamStarted(p.C.Request.Context())

	bodyJSON, _ := json.Marshal(p.Body)
	estimatedInputTokens := len(bodyJSON) / 3
	var inputPrice, outputPrice float64
	if mp := relay.LookupModelPrice(p.TargetModel, p.C.Request.Context(), h.DB); mp != nil {
		inputPrice = mp.InputPrice
		outputPrice = mp.OutputPrice
	}
	streamStartPayload := map[string]any{
		"streamId":             streamId,
		"requestModelName":     p.RequestModel,
		"actualModelName":      p.TargetModel,
		"channelId":            p.Channel.ID,
		"channelName":          p.Channel.Name,
		"time":                 time.Now().Unix(),
		"estimatedInputTokens": estimatedInputTokens,
		"inputPrice":           inputPrice,
		"outputPrice":          outputPrice,
		"requestContent":       string(bodyJSON),
	}
	if h.Broadcast != nil {
		h.Broadcast("log-stream-start", streamStartPayload)
	}
	if h.StreamTracker != nil {
		h.StreamTracker.TrackStream(streamId, streamStartPayload)
	}

	var onContent relay.StreamContentCallback
	if h.Broadcast != nil {
		onContent = func(thinking, response string) {
			h.Broadcast("log-streaming", map[string]any{
				"streamId":        streamId,
				"thinkingContent": thinking,
				"responseContent": response,
				"thinkingLength":  len(thinking),
				"responseLength":  len(response),
			})
		}
	}

	streamInfo, proxyErr := h.CopilotRelay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		accessToken,
		p.TargetModel,
		p.Body,
		p.Channel.Type,
		p.IsAnthropicInbound,
		onContent,
	)
	if proxyErr != nil {
		return &relayResult{StreamID: streamId}, proxyErr
	}

	return &relayResult{
		InputTokens:     streamInfo.InputTokens,
		OutputTokens:    streamInfo.OutputTokens,
		FirstTokenTime:  streamInfo.FirstTokenTime,
		ResponseContent: streamInfo.ResponseContent,
		ThinkingContent: streamInfo.ThinkingContent,
		StreamID:        streamId,
		ResponseHeaders: streamInfo.UpstreamHeaders,
	}, nil
}

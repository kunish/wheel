package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	sdkcliproxy "github.com/kunish/wheel/apps/worker/internal/runtime/sdk/cliproxy"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/kunish/wheel/apps/worker/internal/types"
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
	copilotRelayOpenAIIntent  = "conversation-panel"
	copilotRelayGitHubAPIVer  = "2025-04-01"
)

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

	url := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCopilotHeaders(req, apiToken, bodyJSON)

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

	url := baseURL + "/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCopilotHeaders(req, apiToken, bodyJSON)

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
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)

	// When the inbound is Anthropic format, convert OpenAI SSE to Anthropic SSE.
	var convertToAnthropic func(string) []string
	if anthropicInbound {
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
		if convertToAnthropic != nil {
			// Anthropic mode: convert data: lines through the SSE converter.
			if bytes.HasPrefix(line, []byte("data: ")) {
				data := string(line[6:])
				converted := convertToAnthropic(data)
				for _, l := range converted {
					fmt.Fprintf(w, "%s\n", l)
				}
				if flusher != nil {
					flusher.Flush()
				}
			}
			// Non-data lines (empty lines, etc.) are skipped;
			// the converter handles Anthropic event formatting internally.
		} else {
			// OpenAI passthrough mode: write raw lines.
			_, _ = w.Write(line)
			_, _ = w.Write([]byte("\n"))
			if flusher != nil {
				flusher.Flush()
			}
		}

		// --- Track first token time ---
		if !firstTokenSent && bytes.HasPrefix(line, []byte("data: ")) {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// --- Extract usage and content from original data ---
		if bytes.HasPrefix(line, []byte("data: ")) {
			chunk := line[6:]
			if !bytes.Equal(chunk, []byte("[DONE]")) {
				var obj map[string]any
				if json.Unmarshal(chunk, &obj) == nil {
					if usage, ok := obj["usage"].(map[string]any); ok {
						inputTokens = toIntVal(usage["prompt_tokens"])
						outputTokens = toIntVal(usage["completion_tokens"])
					}
					// Accumulate content for callback.
					if choices, ok := obj["choices"].([]any); ok && len(choices) > 0 {
						if choice, ok := choices[0].(map[string]any); ok {
							if delta, ok := choice["delta"].(map[string]any); ok {
								if c, ok := delta["content"].(string); ok {
									accContent += c
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

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
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/uptrace/bun"
)

const (
	antigravityDailyURL = "https://daily-cloudcode-pa.sandbox.googleapis.com"
	antigravityProdURL  = "https://cloudcode-pa.googleapis.com"
	antigravityUA       = "antigravity/1.15.8 windows/amd64"
)

// AntigravityRelay handles Antigravity (Google Cloud Code) channel requests
// by converting Anthropic Messages API requests into Gemini internal format
// and proxying to Google's internal cloudcode-pa endpoint.
type AntigravityRelay struct {
	db      *bun.DB
	tokenMu sync.RWMutex
}

// NewAntigravityRelay creates a new AntigravityRelay with the given DB for auth file lookup.
func NewAntigravityRelay(db *bun.DB) *AntigravityRelay {
	return &AntigravityRelay{db: db}
}

// ResolveAccessToken maps a channel key to the actual Google OAuth access_token
// from the auth files stored in the database.
func (r *AntigravityRelay) ResolveAccessToken(ctx context.Context, channelID int, channelKey string) (accessToken string, projectID string, err error) {
	items, err := dal.ListCodexAuthFiles(ctx, r.db, channelID)
	if err != nil {
		return "", "", fmt.Errorf("load antigravity auth files: %w", err)
	}

	for _, item := range items {
		if item.Disabled {
			continue
		}
		managedName := codexruntime.ManagedAuthFileName(item.ChannelID, item.Name)
		authIndex := runtimeauth.EnsureAuthIndex(managedName, "", "")
		if authIndex == "" {
			authIndex = item.Name
		}
		if authIndex != channelKey {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal([]byte(item.Content), &raw); err != nil {
			return "", "", fmt.Errorf("parse antigravity auth file content: %w", err)
		}
		token, _ := raw["access_token"].(string)
		if token == "" {
			return "", "", fmt.Errorf("antigravity auth file %q has no access_token", item.Name)
		}
		projID, _ := raw["project_id"].(string)
		return token, projID, nil
	}

	return "", "", fmt.Errorf("no antigravity auth file matches channel key %q", channelKey)
}

// antigravityBaseURL selects the upstream base URL. For Claude models, prefer
// the production endpoint; for Gemini models, use the daily endpoint.
func antigravityBaseURL(model string) string {
	if strings.Contains(model, "claude") {
		return antigravityProdURL
	}
	return antigravityDailyURL
}

// ProxyNonStreaming executes a non-streaming Antigravity API request.
// It converts the Anthropic-format request body to Gemini envelope format,
// sends it upstream, and converts the response back to Anthropic format.
func (r *AntigravityRelay) ProxyNonStreaming(
	ctx context.Context,
	accessToken string,
	projectID string,
	model string,
	body map[string]any,
) (*relay.ProxyResult, error) {
	geminiBody := convertAnthropicToGemini(body, model)
	envelope := buildAntigravityEnvelope(geminiBody, model, projectID)
	bodyJSON, _ := json.Marshal(envelope)

	baseURL := antigravityBaseURL(model)
	url := baseURL + "/v1internal:generateContent"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyAntigravityHeaders(req, accessToken, baseURL)

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

	var geminiResp map[string]any
	if err := json.Unmarshal(respBytes, &geminiResp); err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("parse response: %v", err), StatusCode: http.StatusBadGateway}
	}

	anthropicResp, inputTokens, outputTokens := convertGeminiToAnthropic(geminiResp, model)

	return &relay.ProxyResult{
		Response:        anthropicResp,
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		StatusCode:      resp.StatusCode,
		UpstreamHeaders: resp.Header.Clone(),
	}, nil
}

// ProxyStreaming executes a streaming Antigravity API request.
// Converts Anthropic request to Gemini format, streams the response,
// and converts each SSE event back to Anthropic format.
func (r *AntigravityRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	accessToken string,
	projectID string,
	model string,
	body map[string]any,
	onContent relay.StreamContentCallback,
) (*relay.StreamCompleteInfo, error) {
	geminiBody := convertAnthropicToGemini(body, model)
	envelope := buildAntigravityEnvelope(geminiBody, model, projectID)
	bodyJSON, _ := json.Marshal(envelope)

	baseURL := antigravityBaseURL(model)
	url := baseURL + "/v1internal:streamGenerateContent?alt=sse"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyAntigravityHeaders(req, accessToken, baseURL)

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

	// Write Anthropic SSE headers.
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)

	// Send initial message_start event.
	msgStartEvent := map[string]any{
		"type": "message_start",
		"message": map[string]any{
			"id":      fmt.Sprintf("msg_%d", time.Now().UnixNano()),
			"type":    "message",
			"role":    "assistant",
			"model":   model,
			"content": []any{},
			"usage":   map[string]any{"input_tokens": 0, "output_tokens": 0},
		},
	}
	writeAnthropicSSE(w, flusher, "message_start", msgStartEvent)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)

	var inputTokens, outputTokens int
	var firstTokenTime int
	started := time.Now()
	firstTokenSent := false
	var accContent, accThinking string
	contentBlockIdx := 0
	contentBlockStarted := false

	for scanner.Scan() {
		line := scanner.Bytes()

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		chunk := line[6:]

		var geminiChunk map[string]any
		if json.Unmarshal(chunk, &geminiChunk) != nil {
			continue
		}

		// Track first token time.
		if !firstTokenSent {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// Extract usage metadata.
		if usage, ok := geminiChunk["usageMetadata"].(map[string]any); ok {
			inputTokens = toIntVal(usage["promptTokenCount"])
			outputTokens = toIntVal(usage["candidatesTokenCount"])
		}

		// Process candidates.
		candidates, _ := geminiChunk["candidates"].([]any)
		if len(candidates) == 0 {
			continue
		}
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)

		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}

			// Handle thought (thinking) parts.
			if thought, ok := part["thought"].(bool); ok && thought {
				if text, ok := part["text"].(string); ok && text != "" {
					if !contentBlockStarted {
						writeAnthropicSSE(w, flusher, "content_block_start", map[string]any{
							"type":          "content_block_start",
							"index":         contentBlockIdx,
							"content_block": map[string]any{"type": "thinking", "thinking": ""},
						})
						contentBlockStarted = true
					}
					writeAnthropicSSE(w, flusher, "content_block_delta", map[string]any{
						"type":  "content_block_delta",
						"index": contentBlockIdx,
						"delta": map[string]any{"type": "thinking_delta", "thinking": text},
					})
					accThinking += text
				}
				continue
			}

			// Handle text parts.
			if text, ok := part["text"].(string); ok && text != "" {
				// Close thinking block if it was open.
				if contentBlockStarted {
					writeAnthropicSSE(w, flusher, "content_block_stop", map[string]any{
						"type":  "content_block_stop",
						"index": contentBlockIdx,
					})
					contentBlockIdx++
					contentBlockStarted = false
				}
				// Start text block if needed.
				if !contentBlockStarted {
					writeAnthropicSSE(w, flusher, "content_block_start", map[string]any{
						"type":          "content_block_start",
						"index":         contentBlockIdx,
						"content_block": map[string]any{"type": "text", "text": ""},
					})
					contentBlockStarted = true
				}
				writeAnthropicSSE(w, flusher, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": contentBlockIdx,
					"delta": map[string]any{"type": "text_delta", "text": text},
				})
				accContent += text
			}

			// Handle function calls.
			if fc, ok := part["functionCall"].(map[string]any); ok {
				if contentBlockStarted {
					writeAnthropicSSE(w, flusher, "content_block_stop", map[string]any{
						"type":  "content_block_stop",
						"index": contentBlockIdx,
					})
					contentBlockIdx++
					contentBlockStarted = false
				}
				fcName, _ := fc["name"].(string)
				fcArgs, _ := fc["args"].(map[string]any)
				fcID := fmt.Sprintf("toolu_%d", contentBlockIdx)
				writeAnthropicSSE(w, flusher, "content_block_start", map[string]any{
					"type":  "content_block_start",
					"index": contentBlockIdx,
					"content_block": map[string]any{
						"type":  "tool_use",
						"id":    fcID,
						"name":  fcName,
						"input": map[string]any{},
					},
				})
				argsJSON, _ := json.Marshal(fcArgs)
				writeAnthropicSSE(w, flusher, "content_block_delta", map[string]any{
					"type":  "content_block_delta",
					"index": contentBlockIdx,
					"delta": map[string]any{
						"type":         "input_json_delta",
						"partial_json": string(argsJSON),
					},
				})
				writeAnthropicSSE(w, flusher, "content_block_stop", map[string]any{
					"type":  "content_block_stop",
					"index": contentBlockIdx,
				})
				contentBlockIdx++
			}
		}
	}
	_ = resp.Body.Close()

	// Close any open content block.
	if contentBlockStarted {
		writeAnthropicSSE(w, flusher, "content_block_stop", map[string]any{
			"type":  "content_block_stop",
			"index": contentBlockIdx,
		})
	}

	// Send message_delta and message_stop.
	writeAnthropicSSE(w, flusher, "message_delta", map[string]any{
		"type":  "message_delta",
		"delta": map[string]any{"stop_reason": "end_turn"},
		"usage": map[string]any{"output_tokens": outputTokens},
	})
	writeAnthropicSSE(w, flusher, "message_stop", map[string]any{
		"type": "message_stop",
	})

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

// writeAnthropicSSE writes a single SSE event in Anthropic format.
func writeAnthropicSSE(w http.ResponseWriter, flusher http.Flusher, eventType string, data any) {
	dataJSON, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", eventType, string(dataJSON))
	if flusher != nil {
		flusher.Flush()
	}
}

// applyAntigravityHeaders sets the required headers for the Antigravity API.
func applyAntigravityHeaders(r *http.Request, accessToken, baseURL string) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+accessToken)
	r.Header.Set("User-Agent", antigravityUA)
	r.Header.Set("Accept-Encoding", "gzip")

	// Extract host from baseURL.
	host := strings.TrimPrefix(baseURL, "https://")
	host = strings.TrimPrefix(host, "http://")
	r.Header.Set("Host", host)
}

// buildAntigravityEnvelope wraps a Gemini request body in the Antigravity envelope format.
func buildAntigravityEnvelope(geminiBody map[string]any, model, projectID string) map[string]any {
	sessionID := fmt.Sprintf("sess-%d", time.Now().UnixNano())
	requestID := fmt.Sprintf("req-%d", time.Now().UnixNano())

	if projectID == "" {
		projectID = "ag-default"
	}

	return map[string]any{
		"project":   projectID,
		"requestId": requestID,
		"model":     model,
		"userAgent": "antigravity",
		"request":   geminiBody,
		"sessionId": sessionID,
	}
}

// ──────────────────────────────────────────────────────────────
// Anthropic → Gemini request conversion
// ──────────────────────────────────────────────────────────────

// convertAnthropicToGemini converts an Anthropic Messages API request body
// to the Gemini generateContent format used by Antigravity.
func convertAnthropicToGemini(body map[string]any, model string) map[string]any {
	result := map[string]any{}

	// Convert messages to Gemini contents.
	messages, _ := body["messages"].([]any)
	var contents []any
	for _, m := range messages {
		msg, ok := m.(map[string]any)
		if !ok {
			continue
		}
		role, _ := msg["role"].(string)
		geminiRole := "user"
		if role == "assistant" {
			geminiRole = "model"
		}

		parts := convertAnthropicContentToGeminiParts(msg["content"], role)
		if len(parts) > 0 {
			contents = append(contents, map[string]any{
				"role":  geminiRole,
				"parts": parts,
			})
		}
	}
	result["contents"] = contents

	// System instruction.
	if sys := body["system"]; sys != nil {
		sysText := extractSystemText(sys)
		if sysText != "" {
			result["systemInstruction"] = map[string]any{
				"parts": []any{map[string]any{"text": sysText}},
			}
		}
	}

	// Generation config.
	genConfig := map[string]any{}
	if t, ok := body["temperature"]; ok {
		genConfig["temperature"] = t
	}
	if tp, ok := body["top_p"]; ok {
		genConfig["topP"] = tp
	}
	if mt, ok := body["max_tokens"]; ok {
		genConfig["maxOutputTokens"] = mt
	}
	if ss, ok := body["stop_sequences"]; ok {
		genConfig["stopSequences"] = ss
	}

	// Thinking config from Anthropic thinking parameter.
	if thinking, ok := body["thinking"].(map[string]any); ok {
		if thinkType, _ := thinking["type"].(string); thinkType == "enabled" {
			budget := toIntVal(thinking["budget_tokens"])
			if budget > 0 {
				genConfig["thinkingConfig"] = map[string]any{
					"thinkingBudget":  budget,
					"includeThoughts": true,
				}
			}
		}
	}

	if len(genConfig) > 0 {
		result["generationConfig"] = genConfig
	}

	// Convert tools.
	if tools, ok := body["tools"].([]any); ok && len(tools) > 0 {
		result["tools"] = convertAnthropicToolsToGemini(tools)
	}

	// Tool choice / tool config.
	if tc := body["tool_choice"]; tc != nil {
		result["toolConfig"] = convertAnthropicToolChoiceToGemini(tc)
	}

	return result
}

// convertAnthropicContentToGeminiParts converts Anthropic message content to Gemini parts.
func convertAnthropicContentToGeminiParts(content any, role string) []any {
	// Simple string content.
	if s, ok := content.(string); ok {
		return []any{map[string]any{"text": s}}
	}

	blocks, ok := content.([]any)
	if !ok {
		return nil
	}

	var parts []any
	for _, b := range blocks {
		blk, ok := b.(map[string]any)
		if !ok {
			continue
		}
		bType, _ := blk["type"].(string)
		switch bType {
		case "text":
			if t, ok := blk["text"].(string); ok {
				parts = append(parts, map[string]any{"text": t})
			}
		case "thinking":
			if t, ok := blk["thinking"].(string); ok {
				part := map[string]any{"text": t, "thought": true}
				if sig, ok := blk["signature"].(string); ok {
					part["thoughtSignature"] = sig
				}
				parts = append(parts, part)
			}
		case "tool_use":
			name, _ := blk["name"].(string)
			input, _ := blk["input"].(map[string]any)
			if input == nil {
				input = map[string]any{}
			}
			part := map[string]any{
				"functionCall": map[string]any{
					"name": name,
					"args": input,
				},
			}
			parts = append(parts, part)
		case "tool_result":
			toolUseID, _ := blk["tool_use_id"].(string)
			resultContent := extractToolResultContent(blk["content"])
			parts = append(parts, map[string]any{
				"functionResponse": map[string]any{
					"id":   toolUseID,
					"name": toolUseID, // Gemini expects name, use ID as fallback
					"response": map[string]any{
						"output": resultContent,
					},
				},
			})
		case "image":
			if src, ok := blk["source"].(map[string]any); ok {
				mediaType, _ := src["media_type"].(string)
				data, _ := src["data"].(string)
				parts = append(parts, map[string]any{
					"inlineData": map[string]any{
						"mimeType": mediaType,
						"data":     data,
					},
				})
			}
		}
	}
	return parts
}

// extractSystemText extracts system text from various Anthropic system formats.
func extractSystemText(sys any) string {
	if s, ok := sys.(string); ok {
		return s
	}
	if blocks, ok := sys.([]any); ok {
		var parts []string
		for _, b := range blocks {
			if m, ok := b.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					parts = append(parts, t)
				}
			}
		}
		return strings.Join(parts, "\n")
	}
	b, _ := json.Marshal(sys)
	return string(b)
}

// extractToolResultContent extracts text from tool_result content.
func extractToolResultContent(content any) string {
	if s, ok := content.(string); ok {
		return s
	}
	if blocks, ok := content.([]any); ok {
		var texts []string
		for _, b := range blocks {
			if m, ok := b.(map[string]any); ok {
				if t, ok := m["text"].(string); ok {
					texts = append(texts, t)
				}
			}
		}
		return strings.Join(texts, "")
	}
	if content != nil {
		b, _ := json.Marshal(content)
		return string(b)
	}
	return ""
}

// convertAnthropicToolsToGemini converts Anthropic tools to Gemini function declarations.
func convertAnthropicToolsToGemini(tools []any) []any {
	var declarations []any
	for _, t := range tools {
		tool, ok := t.(map[string]any)
		if !ok {
			continue
		}
		name, _ := tool["name"].(string)
		desc, _ := tool["description"].(string)
		schema := tool["input_schema"]
		if schema == nil {
			schema = map[string]any{"type": "object", "properties": map[string]any{}}
		}
		declarations = append(declarations, map[string]any{
			"name":                 name,
			"description":          desc,
			"parametersJsonSchema": schema,
		})
	}
	return []any{map[string]any{"functionDeclarations": declarations}}
}

// convertAnthropicToolChoiceToGemini converts Anthropic tool_choice to Gemini toolConfig.
func convertAnthropicToolChoiceToGemini(tc any) map[string]any {
	if s, ok := tc.(string); ok {
		switch s {
		case "auto":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		case "any":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
		case "none":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}
		}
	}
	if m, ok := tc.(map[string]any); ok {
		tcType, _ := m["type"].(string)
		switch tcType {
		case "auto":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
		case "any":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "ANY"}}
		case "none":
			return map[string]any{"functionCallingConfig": map[string]any{"mode": "NONE"}}
		case "tool":
			name, _ := m["name"].(string)
			return map[string]any{"functionCallingConfig": map[string]any{
				"mode":                 "ANY",
				"allowedFunctionNames": []string{name},
			}}
		}
	}
	return map[string]any{"functionCallingConfig": map[string]any{"mode": "AUTO"}}
}

// ──────────────────────────────────────────────────────────────
// Gemini → Anthropic response conversion
// ──────────────────────────────────────────────────────────────

// convertGeminiToAnthropic converts a Gemini response to Anthropic Messages format.
func convertGeminiToAnthropic(geminiResp map[string]any, model string) (map[string]any, int, int) {
	var contentBlocks []any
	var inputTokens, outputTokens int

	// Extract usage.
	if usage, ok := geminiResp["usageMetadata"].(map[string]any); ok {
		inputTokens = toIntVal(usage["promptTokenCount"])
		outputTokens = toIntVal(usage["candidatesTokenCount"])
	}

	// Extract content from candidates.
	candidates, _ := geminiResp["candidates"].([]any)
	if len(candidates) > 0 {
		candidate, _ := candidates[0].(map[string]any)
		content, _ := candidate["content"].(map[string]any)
		parts, _ := content["parts"].([]any)

		for _, p := range parts {
			part, ok := p.(map[string]any)
			if !ok {
				continue
			}

			if thought, ok := part["thought"].(bool); ok && thought {
				if text, ok := part["text"].(string); ok {
					block := map[string]any{"type": "thinking", "thinking": text}
					if sig, ok := part["thoughtSignature"].(string); ok {
						block["signature"] = sig
					}
					contentBlocks = append(contentBlocks, block)
				}
				continue
			}

			if text, ok := part["text"].(string); ok {
				contentBlocks = append(contentBlocks, map[string]any{
					"type": "text",
					"text": text,
				})
			}

			if fc, ok := part["functionCall"].(map[string]any); ok {
				fcName, _ := fc["name"].(string)
				fcArgs, _ := fc["args"].(map[string]any)
				contentBlocks = append(contentBlocks, map[string]any{
					"type":  "tool_use",
					"id":    fmt.Sprintf("toolu_%d", len(contentBlocks)),
					"name":  fcName,
					"input": fcArgs,
				})
			}
		}
	}

	if len(contentBlocks) == 0 {
		contentBlocks = []any{map[string]any{"type": "text", "text": ""}}
	}

	return map[string]any{
		"id":            fmt.Sprintf("msg_%d", time.Now().UnixNano()),
		"type":          "message",
		"role":          "assistant",
		"model":         model,
		"content":       contentBlocks,
		"stop_reason":   "end_turn",
		"stop_sequence": nil,
		"usage": map[string]any{
			"input_tokens":  inputTokens,
			"output_tokens": outputTokens,
		},
	}, inputTokens, outputTokens
}

// ──────────────────────────────────────────────────────────────
// Strategy execution helpers (called from relay_strategy.go)
// ──────────────────────────────────────────────────────────────

// executeAntigravityNonStreaming is called by the non-stream strategy for Antigravity channels.
func (h *RelayHandler) executeAntigravityNonStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, projectID, err := h.AntigravityRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve antigravity access token: %v", err), StatusCode: http.StatusUnauthorized}
	}

	result, proxyErr := h.AntigravityRelay.ProxyNonStreaming(
		p.C.Request.Context(),
		accessToken,
		projectID,
		p.TargetModel,
		p.Body,
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

// executeAntigravityStreaming is called by the stream strategy for Antigravity channels.
func (h *RelayHandler) executeAntigravityStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, projectID, err := h.AntigravityRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve antigravity access token: %v", err), StatusCode: http.StatusUnauthorized}
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

	streamInfo, proxyErr := h.AntigravityRelay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		accessToken,
		projectID,
		p.TargetModel,
		p.Body,
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

package handler

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"github.com/kunish/wheel/apps/worker/internal/codexruntime"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/runtimeauth"
	"github.com/uptrace/bun"
)

const (
	codexCLIBaseURL   = "https://chatgpt.com"
	codexCLIResponses = "/backend-api/codex/responses"
	codexCLIUserAgent = "codex_cli_rs/0.38.0 (Linux; x86_64) xterm-256color"
)

// CodexCLIRelay handles Codex CLI channel requests by proxying to
// chatgpt.com/backend-api/codex/responses using OpenAI OAuth access tokens
// stored in runtime auth files.
type CodexCLIRelay struct {
	db      *bun.DB
	tokenMu sync.RWMutex
}

// NewCodexCLIRelay creates a new CodexCLIRelay with the given DB for auth file lookup.
func NewCodexCLIRelay(db *bun.DB) *CodexCLIRelay {
	return &CodexCLIRelay{db: db}
}

// ResolveAccessToken maps a channel key (AuthIndex hash) to the actual OAuth access_token
// and account ID by loading auth files for the given channel from the database.
func (r *CodexCLIRelay) ResolveAccessToken(ctx context.Context, channelID int, channelKey string) (accessToken string, accountID string, err error) {
	items, err := dal.ListCodexAuthFiles(ctx, r.db, channelID)
	if err != nil {
		return "", "", fmt.Errorf("load codex-cli auth files: %w", err)
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
			return "", "", fmt.Errorf("parse codex-cli auth file content: %w", err)
		}
		token, _ := raw["access_token"].(string)
		if token == "" {
			return "", "", fmt.Errorf("codex-cli auth file %q has no access_token", item.Name)
		}
		acctID, _ := raw["account_id"].(string)
		return token, acctID, nil
	}

	return "", "", fmt.Errorf("no codex-cli auth file matches channel key %q", channelKey)
}

// ProxyNonStreaming executes a non-streaming Codex CLI API request.
func (r *CodexCLIRelay) ProxyNonStreaming(
	ctx context.Context,
	accessToken string,
	accountID string,
	model string,
	body map[string]any,
) (*relay.ProxyResult, error) {
	outBody := copyCodexCLIBody(body)
	outBody["model"] = model
	outBody["stream"] = false
	outBody["store"] = false
	bodyJSON, _ := json.Marshal(outBody)

	url := codexCLIBaseURL + codexCLIResponses
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCodexCLIHeaders(req, accessToken, accountID)

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
		InputTokens:     toIntVal(usage["input_tokens"]),
		OutputTokens:    toIntVal(usage["output_tokens"]),
		StatusCode:      resp.StatusCode,
		UpstreamHeaders: resp.Header.Clone(),
	}, nil
}

// ProxyStreaming executes a streaming Codex CLI API request.
// The Codex CLI uses the OpenAI Responses API format — SSE events are
// transparently forwarded to the client.
func (r *CodexCLIRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	accessToken string,
	accountID string,
	model string,
	body map[string]any,
	anthropicInbound bool,
	onContent relay.StreamContentCallback,
) (*relay.StreamCompleteInfo, error) {
	outBody := copyCodexCLIBody(body)
	outBody["model"] = model
	outBody["stream"] = true
	outBody["store"] = false
	bodyJSON, _ := json.Marshal(outBody)

	url := codexCLIBaseURL + codexCLIResponses
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyJSON))
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("create request: %v", err), StatusCode: http.StatusBadGateway}
	}
	applyCodexCLIHeaders(req, accessToken, accountID)

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

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, _ := w.(http.Flusher)
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)

	var inputTokens, outputTokens int
	var firstTokenTime int
	started := time.Now()
	firstTokenSent := false
	var accContent string

	sseDoneMarker := []byte("[DONE" + "]")

	for scanner.Scan() {
		line := scanner.Bytes()

		// Write raw SSE line to client (Responses API format — passthrough).
		_, _ = w.Write(line)
		_, _ = w.Write([]byte("\n"))
		if flusher != nil {
			flusher.Flush()
		}

		// Track first token time.
		if !firstTokenSent && bytes.HasPrefix(line, []byte("data: ")) {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// Extract usage from SSE data lines.
		if bytes.HasPrefix(line, []byte("data: ")) {
			chunk := line[6:]
			if !bytes.Equal(chunk, sseDoneMarker) {
				var obj map[string]any
				if json.Unmarshal(chunk, &obj) == nil {
					// Responses API: usage is at top level in response.completed
					if usage, ok := obj["usage"].(map[string]any); ok {
						inputTokens = toIntVal(usage["input_tokens"])
						outputTokens = toIntVal(usage["output_tokens"])
					}
					// Accumulate output text delta content.
					if delta, ok := obj["delta"].(string); ok {
						accContent += delta
					}
				}
			}
		}
	}
	_ = resp.Body.Close()

	if onContent != nil {
		onContent("", accContent)
	}

	return &relay.StreamCompleteInfo{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		FirstTokenTime:  firstTokenTime,
		ResponseContent: accContent,
		UpstreamHeaders: resp.Header.Clone(),
	}, nil
}

// applyCodexCLIHeaders sets the required headers for chatgpt.com Codex API.
func applyCodexCLIHeaders(r *http.Request, accessToken, accountID string) {
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Authorization", "Bearer "+accessToken)
	r.Header.Set("Accept", "text/event-stream")
	r.Header.Set("User-Agent", codexCLIUserAgent)
	r.Header.Set("Host", "chatgpt.com")
	if accountID != "" {
		r.Header.Set("Chatgpt-Account-Id", accountID)
	}
}

// copyCodexCLIBody makes a shallow copy of a JSON body map.
func copyCodexCLIBody(body map[string]any) map[string]any {
	out := make(map[string]any, len(body))
	for k, v := range body {
		out[k] = v
	}
	return out
}

// executeCodexCLINonStreaming is called by the non-stream strategy for Codex CLI channels.
func (h *RelayHandler) executeCodexCLINonStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, accountID, err := h.CodexCLIRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve codex-cli access token: %v", err), StatusCode: http.StatusUnauthorized}
	}

	result, proxyErr := h.CodexCLIRelay.ProxyNonStreaming(
		p.C.Request.Context(),
		accessToken,
		accountID,
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

// executeCodexCLIStreaming is called by the stream strategy for Codex CLI channels.
func (h *RelayHandler) executeCodexCLIStreaming(p *relayAttemptParams) (*relayResult, error) {
	accessToken, accountID, err := h.CodexCLIRelay.ResolveAccessToken(p.C.Request.Context(), p.Channel.ID, p.SelectedKey.ChannelKey)
	if err != nil {
		return nil, &relay.ProxyError{Message: fmt.Sprintf("resolve codex-cli access token: %v", err), StatusCode: http.StatusUnauthorized}
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

	streamInfo, proxyErr := h.CodexCLIRelay.ProxyStreaming(
		p.C.Writer,
		p.C.Request.Context(),
		accessToken,
		accountID,
		p.TargetModel,
		p.Body,
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
		StreamID:        streamId,
		ResponseHeaders: streamInfo.UpstreamHeaders,
	}, nil
}

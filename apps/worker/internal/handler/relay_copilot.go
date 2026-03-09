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

	var inputTokens, outputTokens int
	var firstTokenTime int
	started := time.Now()
	firstTokenSent := false
	var accContent, accThinking string

	for scanner.Scan() {
		line := scanner.Bytes()
		_, _ = w.Write(line)
		_, _ = w.Write([]byte("\n"))
		if flusher != nil {
			flusher.Flush()
		}

		if !firstTokenSent && bytes.HasPrefix(line, []byte("data: ")) {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// Try to extract usage from final chunk.
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

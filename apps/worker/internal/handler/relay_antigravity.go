package handler

// relay_antigravity.go is the main entry point for the Antigravity relay proxy.
// It handles auth resolution, HTTP proxying, and delegates conversion to the
// transformer files:
//   - antigravity_types.go: Strongly-typed structs and constants
//   - relay_antigravity_request.go: Anthropic → Gemini request conversion
//   - relay_antigravity_response.go: Gemini → Anthropic non-streaming response conversion
//   - relay_antigravity_stream.go: Gemini → Anthropic streaming response conversion

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
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

// ResolveAccessToken maps a channel key to a fresh Google OAuth access_token.
// It first tries reading the managed auth file from disk (which contains the
// latest refreshed token from the runtime), falling back to the database.
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

		// Try reading the managed file from disk first — the runtime's
		// token refresh mechanism keeps this file up to date.
		if token, projID, diskErr := r.readManagedAuthFile(managedName); diskErr == nil && token != "" {
			return token, projID, nil
		}

		// Fallback: read from database (token may be stale).
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

// readManagedAuthFile reads the token from the managed auth file on disk.
func (r *AntigravityRelay) readManagedAuthFile(managedName string) (string, string, error) {
	filePath := filepath.Join(codexruntime.ManagedAuthDir(), managedName)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", "", err
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", "", err
	}
	token, _ := raw["access_token"].(string)
	projID, _ := raw["project_id"].(string)
	return token, projID, nil
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
	// Use the new typed request transformer.
	envelope := transformClaudeToGemini(body, model, projectID)
	// The model may have been changed by web_search detection.
	effectiveModel := envelope.Model
	bodyJSON, _ := json.Marshal(envelope)

	baseURL := antigravityBaseURL(effectiveModel)
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

	// Use the new typed response transformer.
	anthropicResp, inputTokens, outputTokens := transformGeminiToClaudeResponse(respBytes, effectiveModel)

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
// and converts each SSE event back to Anthropic format using StreamingProcessor.
func (r *AntigravityRelay) ProxyStreaming(
	w http.ResponseWriter,
	ctx context.Context,
	accessToken string,
	projectID string,
	model string,
	body map[string]any,
	onContent relay.StreamContentCallback,
) (*relay.StreamCompleteInfo, error) {
	// Use the new typed request transformer.
	envelope := transformClaudeToGemini(body, model, projectID)
	effectiveModel := envelope.Model
	bodyJSON, _ := json.Marshal(envelope)

	baseURL := antigravityBaseURL(effectiveModel)
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

	// Use the new StreamingProcessor state machine.
	sp := NewStreamingProcessor(w, effectiveModel)

	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 256*1024), 1024*1024)

	var firstTokenTime int
	started := time.Now()
	firstTokenSent := false

	for scanner.Scan() {
		line := scanner.Bytes()

		if !bytes.HasPrefix(line, []byte("data: ")) {
			continue
		}
		chunk := line[6:]

		// Track first token time.
		if !firstTokenSent {
			firstTokenTime = int(time.Since(started).Milliseconds())
			firstTokenSent = true
		}

		// Delegate to the StreamingProcessor.
		sp.ProcessChunk(chunk)
	}
	_ = resp.Body.Close()

	// Finalize the stream.
	inputTokens, outputTokens := sp.Finish()

	if onContent != nil {
		onContent(sp.AccThinking(), sp.AccText())
	}

	return &relay.StreamCompleteInfo{
		InputTokens:     inputTokens,
		OutputTokens:    outputTokens,
		FirstTokenTime:  firstTokenTime,
		ResponseContent: sp.AccText(),
		ThinkingContent: sp.AccThinking(),
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

package relay

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kunish/wheel/apps/worker/internal/types"
)

// RerankRequest represents a rerank API request.
type RerankRequest struct {
	Model     string   `json:"model"`
	Query     string   `json:"query"`
	Documents []string `json:"documents"`
	TopN      int      `json:"top_n,omitempty"`
}

// rerankResult represents a single reranking result.
type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
	Document       string  `json:"document,omitempty"`
}

// RerankResponse represents the rerank API response.
type RerankResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Results []rerankResult `json:"results"`
	Model   string         `json:"model"`
	Usage   map[string]any `json:"usage,omitempty"`
}

// ProxyRerank forwards a rerank request to the upstream provider.
func ProxyRerank(
	client *http.Client,
	upstreamUrl string,
	upstreamHeaders map[string]string,
	body string,
	channelType types.OutboundType,
) (*RerankResponse, error) {
	req, err := http.NewRequest("POST", upstreamUrl, strings.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create rerank request: %w", err)
	}
	for k, v := range upstreamHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("upstream rerank request failed: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read rerank response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &ProxyError{
			Message:    fmt.Sprintf("Upstream rerank error %d: %s", resp.StatusCode, string(bodyBytes)),
			StatusCode: resp.StatusCode,
		}
	}

	var result RerankResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("failed to parse rerank response: %w", err)
	}
	result.Object = "rerank"
	return &result, nil
}

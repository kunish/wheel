package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// HandleRerank handles POST /v1/rerank requests.
func (h *RelayHandler) HandleRerank(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req relay.RerankRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Model == "" || req.Query == "" || len(req.Documents) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model, query, and documents are required"})
		return
	}

	supportedModels, _ := c.Get("supportedModels")
	sm, _ := supportedModels.(string)
	if !middleware.CheckModelAccess(sm, req.Model) {
		c.JSON(http.StatusForbidden, gin.H{"error": "model not allowed for this API key"})
		return
	}

	apiKeyIDRaw, _ := c.Get("apiKeyId")
	apiKeyID, _ := apiKeyIDRaw.(int)

	var result *relay.RerankResponse
	err = h.executeFeatureWithRetry(req.Model, apiKeyID, func(channel *types.Channel, selectedKey *types.ChannelKey, targetModel string) error {
		baseURL := relay.SelectBaseUrl([]types.BaseUrl(channel.BaseUrls), channel.Type)
		upstreamURL := baseURL + "/v1/rerank"
		if channel.Type == types.OutboundCohere {
			upstreamURL = baseURL + "/v2/rerank"
		}

		payloadBody, marshalErr := buildRerankPayload(req, targetModel)
		if marshalErr != nil {
			return marshalErr
		}

		headers := map[string]string{"Content-Type": "application/json"}
		switch channel.Type {
		case types.OutboundAnthropic, types.OutboundBedrock:
			headers["x-api-key"] = selectedKey.ChannelKey
			headers["anthropic-version"] = "2023-06-01"
		case types.OutboundAzureOpenAI:
			headers["api-key"] = selectedKey.ChannelKey
		default:
			headers["Authorization"] = "Bearer " + selectedKey.ChannelKey
		}
		for _, ch := range channel.CustomHeader {
			headers[ch.Key] = ch.Value
		}

		resp, proxyErr := relay.ProxyRerank(h.HTTPClient, upstreamURL, headers, payloadBody, channel.Type)
		if proxyErr != nil {
			return proxyErr
		}
		if resp.Model == "" {
			resp.Model = targetModel
		}
		result = resp
		return nil
	})
	if err != nil {
		c.JSON(http.StatusBadGateway, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func buildRerankPayload(req relay.RerankRequest, targetModel string) (string, error) {
	payload := req
	payload.Model = targetModel
	b, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

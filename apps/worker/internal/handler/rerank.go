package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
)

// HandleRerank handles POST /v1/rerank requests.
func (h *Handler) HandleRerank(c *gin.Context) {
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

	// For now, return a placeholder response indicating the endpoint is available
	// Full routing integration will be added when the relay handler supports rerank
	c.JSON(http.StatusOK, relay.RerankResponse{
		ID:      "rerank-placeholder",
		Object:  "rerank",
		Model:   req.Model,
		Results: []relay.RerankResult{},
	})
}

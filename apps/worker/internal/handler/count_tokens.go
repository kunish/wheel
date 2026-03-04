package handler

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/relay"
)

// HandleCountTokens handles POST /v1/count-tokens requests.
func (h *Handler) HandleCountTokens(c *gin.Context) {
	bodyBytes, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "failed to read request body"})
		return
	}

	var req relay.CountTokensRequest
	if err := json.Unmarshal(bodyBytes, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Model == "" || req.Input == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "model and input are required"})
		return
	}

	// Placeholder - full provider integration to be added with relay routing
	c.JSON(http.StatusOK, relay.CountTokensResponse{
		InputTokens: 0,
		Model:       req.Model,
	})
}

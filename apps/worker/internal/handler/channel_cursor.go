package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// RefreshCursorChannelModels calls Cursor GetUsableModels with the saved channel
// credentials and persists the merged model list (like Codex sync model refresh).
func (h *Handler) RefreshCursorChannelModels(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		errorJSON(c, http.StatusBadRequest, "invalid channel id")
		return
	}
	ctx := c.Request.Context()
	channel, err := dal.GetChannel(ctx, h.DB, id)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if channel == nil {
		errorJSON(c, http.StatusNotFound, "Channel not found")
		return
	}
	if channel.Type != types.OutboundCursor {
		errorJSON(c, http.StatusBadRequest, "channel is not a Cursor IDE channel")
		return
	}

	models, _, err := fetchModelsFromChannel(ctx, channel, h.Cache)
	if err != nil {
		errorJSON(c, http.StatusBadGateway, err.Error())
		return
	}
	if len(models) == 0 {
		models = cursorFallbackUsableModelIDs()
	}
	if len(models) == 0 {
		successJSON(c, map[string]any{
			"models":    []string(channel.Model),
			"unchanged": true,
		})
		return
	}

	merged := mergeChannelModelIDs([]string(channel.Model), models)
	if err := h.persistCodexChannelModels(ctx, id, merged); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	h.Cache.Delete("channels")
	successJSON(c, map[string]any{"models": merged, "count": len(merged)})
}

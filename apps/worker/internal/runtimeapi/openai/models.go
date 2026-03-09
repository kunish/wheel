package openai

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (h *APIHandler) OpenAIModels(c *gin.Context) {
	models := []map[string]any(nil)
	if h != nil && h.models != nil {
		models = h.models()
	} else if h != nil && h.inner != nil {
		models = h.inner.Models()
	}
	c.JSON(http.StatusOK, gin.H{
		"object": "list",
		"data":   FilterOpenAIModels(models),
	})
}

func FilterOpenAIModels(allModels []map[string]any) []map[string]any {
	filteredModels := make([]map[string]any, len(allModels))
	for i, model := range allModels {
		filteredModel := map[string]any{
			"id":     model["id"],
			"object": model["object"],
		}
		if created, exists := model["created"]; exists {
			filteredModel["created"] = created
		}
		if ownedBy, exists := model["owned_by"]; exists {
			filteredModel["owned_by"] = ownedBy
		}
		filteredModels[i] = filteredModel
	}
	return filteredModels
}

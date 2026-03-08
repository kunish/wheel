package openai

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/router-for-me/CLIProxyAPI/v6/sdk/api/handlers"
	"github.com/tidwall/gjson"
)

func requireOpenAIModel(c *gin.Context, rawJSON []byte) (string, bool) {
	modelName := gjson.GetBytes(rawJSON, "model").String()
	if modelName == "" {
		c.JSON(http.StatusBadRequest, handlers.ErrorResponse{
			Error: handlers.ErrorDetail{
				Message: "Model is required",
				Type:    "invalid_request_error",
			},
		})
		return "", false
	}
	return modelName, true
}

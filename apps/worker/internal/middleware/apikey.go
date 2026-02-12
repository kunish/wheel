package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/uptrace/bun"
)

// ApiKeyAuth is a Gin middleware that validates API keys from x-api-key header
// or Authorization: Bearer sk-wheel-* header.
func ApiKeyAuth(db *bun.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := c.GetHeader("x-api-key")
		if key == "" {
			authHeader := c.GetHeader("Authorization")
			if strings.HasPrefix(authHeader, "Bearer sk-wheel-") {
				key = authHeader[7:]
			}
		}

		if key == "" || !strings.HasPrefix(key, "sk-wheel-") {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized: invalid API key"})
			c.Abort()
			return
		}

		apiKey, err := dal.GetApiKeyByKey(c.Request.Context(), db, key)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": "Internal server error"})
			c.Abort()
			return
		}
		if apiKey == nil {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized: API key not found"})
			c.Abort()
			return
		}

		if !apiKey.Enabled {
			c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": "Unauthorized: API key disabled"})
			c.Abort()
			return
		}

		// Check expiry
		if apiKey.ExpireAt > 0 && apiKey.ExpireAt < time.Now().Unix() {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Forbidden: API key expired"})
			c.Abort()
			return
		}

		// Check cost limit
		if apiKey.MaxCost > 0 && apiKey.TotalCost >= apiKey.MaxCost {
			c.JSON(http.StatusForbidden, gin.H{"success": false, "error": "Forbidden: cost limit exceeded"})
			c.Abort()
			return
		}

		c.Set("apiKeyId", apiKey.ID)
		c.Set("supportedModels", apiKey.SupportedModels)

		c.Next()
	}
}

// CheckModelAccess checks if a model is in the allowed list.
func CheckModelAccess(supportedModels, model string) bool {
	if supportedModels == "" {
		return true
	}
	for _, m := range strings.Split(supportedModels, ",") {
		if strings.TrimSpace(m) == model {
			return true
		}
	}
	return false
}

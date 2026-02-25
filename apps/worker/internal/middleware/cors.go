package middleware

import (
	"net/http"
	"slices"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles CORS headers. When allowedOrigins is non-empty,
// only listed origins are permitted; otherwise all origins are allowed (dev mode).
func CORSMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")

		if len(allowedOrigins) == 0 {
			// Dev mode: allow all origins
			c.Header("Access-Control-Allow-Origin", "*")
		} else if origin != "" && slices.Contains(allowedOrigins, origin) {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Vary", "Origin")
		} else {
			// Origin not in whitelist — skip CORS headers entirely.
			// Security headers still apply.
			c.Header("X-Content-Type-Options", "nosniff")
			c.Header("X-Frame-Options", "DENY")

			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusNoContent)
				return
			}
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Accept, Authorization, X-Api-Key")
		c.Header("Access-Control-Max-Age", "86400")

		// Security headers
		c.Header("X-Content-Type-Options", "nosniff")
		c.Header("X-Frame-Options", "DENY")

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

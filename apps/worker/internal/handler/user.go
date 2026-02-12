package handler

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/uptrace/bun"
)

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	DB     *bun.DB
	LogDB  *bun.DB
	Cache  *cache.MemoryKV
	Config *config.Config
}

// JSON helpers

func successJSON(c *gin.Context, data interface{}) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func successNoData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func errorJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

// ──── User Routes ────

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *Handler) Login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Use constant-time comparison to prevent timing attacks
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.Config.AdminUsername))
	passwordMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.Config.AdminPassword))
	if usernameMatch&passwordMatch != 1 {
		errorJSON(c, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	payload, expireAt := middleware.GenerateToken(-1) // 30 days
	token, err := middleware.SignJWT(payload, h.Config.JWTSecret)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, "Failed to generate token")
		return
	}

	successJSON(c, gin.H{"token": token, "expireAt": expireAt})
}

type changePasswordRequest struct {
	OldPassword string `json:"oldPassword"`
	NewPassword string `json:"newPassword"`
}

func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if subtle.ConstantTimeCompare([]byte(req.OldPassword), []byte(h.Config.AdminPassword)) != 1 {
		errorJSON(c, http.StatusUnauthorized, "Old password is incorrect")
		return
	}

	h.Config.AdminPassword = req.NewPassword
	successNoData(c)
}

type changeUsernameRequest struct {
	Username string `json:"username"`
}

func (h *Handler) ChangeUsername(c *gin.Context) {
	var req changeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.Config.AdminUsername = req.Username
	successNoData(c)
}

func (h *Handler) UserStatus(c *gin.Context) {
	successJSON(c, gin.H{"authenticated": true})
}

package handler

import (
	"crypto/subtle"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
)

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	DB     *bun.DB
	Cache  *cache.MemoryKV
	Config *config.Config
}

// JSON helpers

func successJSON(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func successNoData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func errorJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"success": false, "error": msg})
}

// ──── User Routes ────

// Login godoc
// @Summary Admin login
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body types.LoginRequest true "Login credentials"
// @Success 200 {object} object "{success: true, data: {token, expireAt}}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Failure 401 {object} object "{success: false, error: string}"
// @Router /api/v1/user/login [post]
func (h *Handler) Login(c *gin.Context) {
	var req types.LoginRequest
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
	NewPassword string `json:"newPassword"`
}

// ChangePassword godoc
// @Summary Change admin password
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body changePasswordRequest true "Old and new password"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Failure 401 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/user/change-password [post]
func (h *Handler) ChangePassword(c *gin.Context) {
	var req changePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.NewPassword == "" {
		errorJSON(c, http.StatusBadRequest, "New password is required")
		return
	}

	h.Config.AdminPassword = req.NewPassword
	successNoData(c)
}

type changeUsernameRequest struct {
	Username string `json:"username"`
}

// ChangeUsername godoc
// @Summary Change admin username
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body changeUsernameRequest true "New username"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/user/change-username [post]
func (h *Handler) ChangeUsername(c *gin.Context) {
	var req changeUsernameRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	h.Config.AdminUsername = req.Username
	successNoData(c)
}

// UserStatus godoc
// @Summary Check authentication status
// @Tags Auth
// @Produce json
// @Success 200 {object} object "{success: true, data: {authenticated: true}}"
// @Security BearerAuth
// @Router /api/v1/user/status [get]
func (h *Handler) UserStatus(c *gin.Context) {
	successJSON(c, gin.H{"authenticated": true})
}

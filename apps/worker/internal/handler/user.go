package handler

import (
	"crypto/subtle"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/cache"
	"github.com/kunish/wheel/apps/worker/internal/config"
	"github.com/kunish/wheel/apps/worker/internal/db"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/middleware"
	"github.com/kunish/wheel/apps/worker/internal/relay"
	"github.com/kunish/wheel/apps/worker/internal/types"
	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

// Handler holds shared dependencies for all route handlers.
type Handler struct {
	DB              *bun.DB
	Cache           *cache.MemoryKV
	Config          *config.Config
	CircuitBreakers *relay.CircuitBreakerManager
	DLock           *db.DistributedLock
	codexQuotaDo    func(*http.Request) (*http.Response, error)
	// quotaCache stores quota results keyed by "channelID:fileName".
	// Values are quotaCacheEntry. Populated during normal browsing,
	// used for instant status filtering.
	quotaCache sync.Map
}

// JSON helpers

func successJSON(c *gin.Context, data any) {
	c.JSON(http.StatusOK, gin.H{"success": true, "data": data})
}

func successNoData(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"success": true})
}

func errorJSON(c *gin.Context, status int, msg string) {
	if status >= 500 {
		slog.Error("internal error", "status", status, "error", msg, "path", c.Request.URL.Path)
		c.JSON(status, gin.H{"success": false, "error": "Internal server error"})
		return
	}
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

	authenticated := false

	// Try DB user first (set via reset-password CLI)
	if user, err := dal.GetUser(c.Request.Context(), h.DB); err == nil && user != nil {
		if subtle.ConstantTimeCompare([]byte(req.Username), []byte(user.Username)) == 1 &&
			bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)) == nil {
			authenticated = true
		}
	}

	// Fallback to env config
	if !authenticated {
		usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.Config.AdminUsername))
		passwordMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.Config.AdminPassword))
		authenticated = usernameMatch&passwordMatch == 1
	}

	if !authenticated {
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

	// Persist to DB if a DB user exists
	ctx := c.Request.Context()
	if user, err := dal.GetUser(ctx, h.DB); err == nil && user != nil {
		hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, "Failed to hash password")
			return
		}
		if err := dal.UpdatePassword(ctx, h.DB, user.ID, string(hashed)); err != nil {
			errorJSON(c, http.StatusInternalServerError, "Failed to update password")
			return
		}
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

	if req.Username == "" {
		errorJSON(c, http.StatusBadRequest, "Username is required")
		return
	}

	ctx := c.Request.Context()
	if user, err := dal.GetUser(ctx, h.DB); err == nil && user != nil {
		if err := dal.UpdateUsername(ctx, h.DB, user.ID, req.Username); err != nil {
			errorJSON(c, http.StatusInternalServerError, "Failed to update username")
			return
		}
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

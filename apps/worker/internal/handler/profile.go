package handler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Profile Routes ────

// ListProfiles godoc
// @Summary List all model profiles
// @Tags Profiles
// @Produce json
// @Success 200 {object} object "{success: true, data: {profiles: []ModelProfile}}"
// @Security BearerAuth
// @Router /api/v1/model/profiles [get]
func (h *Handler) ListProfiles(c *gin.Context) {
	profiles, err := dal.ListProfiles(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"profiles": profiles})
}

// CreateProfile godoc
// @Summary Create a custom model profile
// @Tags Profiles
// @Accept json
// @Produce json
// @Param body body types.ProfileCreateRequest true "Profile data"
// @Success 200 {object} object "{success: true, data: ModelProfile}"
// @Security BearerAuth
// @Router /api/v1/model/profiles [post]
func (h *Handler) CreateProfile(c *gin.Context) {
	var body types.ProfileCreateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		errorJSON(c, http.StatusBadRequest, "name is required")
		return
	}

	profile, err := dal.CreateProfile(
		c.Request.Context(),
		h.DB,
		strings.TrimSpace(body.Name),
		body.Provider,
		body.Models,
	)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, profile)
}

// UpdateProfile godoc
// @Summary Update a custom model profile
// @Tags Profiles
// @Accept json
// @Produce json
// @Param body body types.ProfileUpdateRequest true "Profile data"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/model/profiles/update [post]
func (h *Handler) UpdateProfile(c *gin.Context) {
	var body types.ProfileUpdateRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}
	if strings.TrimSpace(body.Name) == "" {
		errorJSON(c, http.StatusBadRequest, "name is required")
		return
	}

	existing, err := dal.GetProfile(c.Request.Context(), h.DB, body.ID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		errorJSON(c, http.StatusNotFound, "profile not found")
		return
	}
	if existing.IsBuiltin {
		errorJSON(c, http.StatusForbidden, "builtin profiles cannot be modified")
		return
	}

	if err := dal.UpdateProfile(
		c.Request.Context(),
		h.DB,
		body.ID,
		strings.TrimSpace(body.Name),
		body.Provider,
		body.Models,
		body.Models != nil,
	); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// DeleteProfile godoc
// @Summary Delete a custom model profile
// @Tags Profiles
// @Accept json
// @Produce json
// @Param body body types.ProfileDeleteRequest true "Profile ID"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/model/profiles/delete [post]
func (h *Handler) DeleteProfile(c *gin.Context) {
	var body types.ProfileDeleteRequest
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if body.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	ctx := c.Request.Context()

	existing, err := dal.GetProfile(ctx, h.DB, body.ID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if existing == nil {
		errorJSON(c, http.StatusNotFound, "profile not found")
		return
	}
	if existing.IsBuiltin {
		errorJSON(c, http.StatusForbidden, "builtin profiles cannot be deleted")
		return
	}

	count, err := dal.CountGroupsByProfile(ctx, h.DB, body.ID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if count > 0 {
		errorJSON(c, http.StatusConflict, "profile has groups, move or delete them first")
		return
	}

	if err := dal.DeleteProfile(ctx, h.DB, body.ID); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successNoData(c)
}

// ActivateProfile godoc
// @Summary Set the active model profile
// @Tags Profiles
// @Accept json
// @Produce json
// @Param body body object true "{id: number}"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/model/profiles/activate [post]
func (h *Handler) ActivateProfile(c *gin.Context) {
	var body struct {
		ID int `json:"id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	// id=0 means "all groups" (no profile filter)
	if body.ID != 0 {
		existing, err := dal.GetProfile(c.Request.Context(), h.DB, body.ID)
		if err != nil {
			errorJSON(c, http.StatusInternalServerError, err.Error())
			return
		}
		if existing == nil {
			errorJSON(c, http.StatusNotFound, "profile not found")
			return
		}
	}

	if err := dal.UpdateSettings(c.Request.Context(), h.DB, map[string]string{
		"active_profile_id": fmt.Sprintf("%d", body.ID),
	}); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// Clear caches so relay picks up the new profile filter
	h.Cache.Delete("groups")
	h.Cache.Delete("settings")

	successNoData(c)
}

type profilePreviewGroup struct {
	Key          string `json:"key"`
	Name         string `json:"name"`
	Model        string `json:"model"`
	Virtual      bool   `json:"virtual"`
	Materialized bool   `json:"materialized"`
	GroupID      *int   `json:"groupId,omitempty"`
}

type profileGroupsMaterializeRequest struct {
	Models []string `json:"models"`
}

// ListProfileGroupsPreview godoc
// @Summary List virtual preview groups for a profile
// @Tags Profiles
// @Produce json
// @Param id path int true "Profile ID"
// @Success 200 {object} object "{success: true, data: {groups: []object}}"
// @Security BearerAuth
// @Router /api/v1/model/profiles/{id}/groups-preview [get]
func (h *Handler) ListProfileGroupsPreview(c *gin.Context) {
	profileID, err := strconv.Atoi(c.Param("id"))
	if err != nil || profileID <= 0 {
		errorJSON(c, http.StatusBadRequest, "invalid profile id")
		return
	}

	profile, err := dal.GetProfile(c.Request.Context(), h.DB, profileID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == nil {
		errorJSON(c, http.StatusNotFound, "profile not found")
		return
	}

	models := dal.NormalizeModels([]string(profile.Models))
	groups, err := dal.ListGroupsByProfileLite(c.Request.Context(), h.DB, profileID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	groupIDByName := make(map[string]int, len(groups))
	for _, g := range groups {
		groupIDByName[g.Name] = g.ID
	}

	previewGroups := make([]profilePreviewGroup, 0, len(models))
	for _, model := range models {
		preview := profilePreviewGroup{
			Key:     model,
			Name:    model,
			Model:   model,
			Virtual: true,
		}
		if gid, ok := groupIDByName[model]; ok {
			id := gid
			preview.Materialized = true
			preview.GroupID = &id
		}
		previewGroups = append(previewGroups, preview)
	}

	successJSON(c, gin.H{"groups": previewGroups})
}

// MaterializeProfileGroups godoc
// @Summary Materialize virtual profile groups into real routing groups
// @Tags Profiles
// @Accept json
// @Produce json
// @Param id path int true "Profile ID"
// @Param body body profileGroupsMaterializeRequest false "Selected model names"
// @Success 200 {object} object "{success: true, data: {created: int, existing: int}}"
// @Security BearerAuth
// @Router /api/v1/model/profiles/{id}/groups-materialize [post]
func (h *Handler) MaterializeProfileGroups(c *gin.Context) {
	profileID, err := strconv.Atoi(c.Param("id"))
	if err != nil || profileID <= 0 {
		errorJSON(c, http.StatusBadRequest, "invalid profile id")
		return
	}

	profile, err := dal.GetProfile(c.Request.Context(), h.DB, profileID)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	if profile == nil {
		errorJSON(c, http.StatusNotFound, "profile not found")
		return
	}

	var req profileGroupsMaterializeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	targetModels := req.Models
	if len(targetModels) == 0 {
		targetModels = []string(profile.Models)
	}
	targetModels = dal.NormalizeModels(targetModels)

	created, existing, err := dal.MaterializeProfileGroups(c.Request.Context(), h.DB, profileID, targetModels)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{
		"created":  created,
		"existing": existing,
		"total":    len(targetModels),
	})
}

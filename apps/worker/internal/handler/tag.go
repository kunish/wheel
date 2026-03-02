package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Tag Routes ────

// ListTags godoc
// @Summary List all tags
// @Tags Tags
// @Produce json
// @Success 200 {object} object "{success: true, data: {tags: []Tag}}"
// @Security BearerAuth
// @Router /api/v1/tag/list [get]
func (h *Handler) ListTags(c *gin.Context) {
	tags, err := dal.ListTags(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"tags": tags})
}

// CreateTag godoc
// @Summary Create a new tag
// @Tags Tags
// @Accept json
// @Produce json
// @Param body body types.TagCreateRequest true "Tag"
// @Success 200 {object} object "{success: true, data: Tag}"
// @Security BearerAuth
// @Router /api/v1/tag/create [post]
func (h *Handler) CreateTag(c *gin.Context) {
	var req types.TagCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	tag := &types.Tag{
		Name:        req.Name,
		Color:       req.Color,
		Description: req.Description,
	}

	if err := dal.CreateTag(c.Request.Context(), h.DB, tag); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, tag)
}

// UpdateTag godoc
// @Summary Update a tag
// @Tags Tags
// @Accept json
// @Produce json
// @Param body body types.TagUpdateRequest true "Partial tag fields"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/tag/update [post]
func (h *Handler) UpdateTag(c *gin.Context) {
	var req types.TagUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}
	if req.ID == 0 {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}

	data := make(map[string]any)
	if req.Name != nil {
		data["name"] = *req.Name
	}
	if req.Color != nil {
		data["color"] = *req.Color
	}
	if req.Description != nil {
		data["description"] = *req.Description
	}

	if err := dal.UpdateTag(c.Request.Context(), h.DB, req.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// DeleteTag godoc
// @Summary Delete a tag
// @Tags Tags
// @Produce json
// @Param id path int true "Tag ID"
// @Success 200 {object} object "{success: true}"
// @Security BearerAuth
// @Router /api/v1/tag/delete/{id} [delete]
func (h *Handler) DeleteTag(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid tag ID")
		return
	}

	if err := dal.DeleteTag(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

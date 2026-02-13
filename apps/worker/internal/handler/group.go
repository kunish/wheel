package handler

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ──── Group Routes ────

// ListGroups godoc
// @Summary List all routing groups
// @Tags Groups
// @Produce json
// @Success 200 {object} object "{success: true, data: {groups: []Group}}"
// @Security BearerAuth
// @Router /api/v1/group/list [get]
func (h *Handler) ListGroups(c *gin.Context) {
	groups, err := dal.ListGroups(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"groups": groups})
}

// CreateGroup godoc
// @Summary Create a new routing group
// @Tags Groups
// @Accept json
// @Produce json
// @Param body body types.GroupCreateRequest true "Group configuration"
// @Success 200 {object} object "{success: true, data: Group}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/group/create [post]
func (h *Handler) CreateGroup(c *gin.Context) {
	var req types.GroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	g := types.Group{
		Name:              req.Name,
		Mode:              types.GroupMode(req.Mode),
		FirstTokenTimeOut: req.FirstTokenTimeOut,
		SessionKeepTime:   req.SessionKeepTime,
	}

	created, err := dal.CreateGroup(c.Request.Context(), h.DB, g, req.Items)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("groups")
	successJSON(c, created)
}

// UpdateGroup godoc
// @Summary Update group configuration
// @Tags Groups
// @Accept json
// @Produce json
// @Param body body object true "Partial group fields to update (id required)"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/group/update [post]
func (h *Handler) UpdateGroup(c *gin.Context) {
	var body map[string]interface{}
	if err := c.ShouldBindJSON(&body); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	idFloat, ok := body["id"].(float64)
	if !ok {
		errorJSON(c, http.StatusBadRequest, "id is required")
		return
	}
	id := int(idFloat)

	data := make(map[string]interface{})
	if v, ok := body["name"]; ok {
		data["name"] = v
	}
	if v, ok := body["mode"]; ok {
		data["mode"] = v
	}
	if v, ok := body["firstTokenTimeOut"]; ok {
		data["first_token_time_out"] = v
	}
	if v, ok := body["sessionKeepTime"]; ok {
		data["session_keep_time"] = v
	}

	var items []types.GroupItemInput
	replaceItems := false
	if itemsRaw, ok := body["items"]; ok {
		itemsJSON, _ := json.Marshal(itemsRaw)
		json.Unmarshal(itemsJSON, &items)
		replaceItems = true
	}

	if err := dal.UpdateGroup(c.Request.Context(), h.DB, id, data, items, replaceItems); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("groups")
	successNoData(c)
}

// DeleteGroup godoc
// @Summary Delete a routing group
// @Tags Groups
// @Produce json
// @Param id path int true "Group ID"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/group/delete/{id} [delete]
func (h *Handler) DeleteGroup(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid group ID")
		return
	}

	if err := dal.DeleteGroup(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("groups")
	successNoData(c)
}

type reorderRequest struct {
	OrderedIds []int `json:"orderedIds"`
}

// ReorderGroups godoc
// @Summary Reorder routing groups
// @Tags Groups
// @Accept json
// @Produce json
// @Param body body reorderRequest true "Ordered group IDs"
// @Success 200 {object} object "{success: true}"
// @Failure 400 {object} object "{success: false, error: string}"
// @Security BearerAuth
// @Router /api/v1/group/reorder [post]
func (h *Handler) ReorderGroups(c *gin.Context) {
	var req reorderRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	if err := dal.ReorderGroups(c.Request.Context(), h.DB, req.OrderedIds); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	h.Cache.Delete("groups")
	successNoData(c)
}

// GroupModelList godoc
// @Summary List all models available across channels
// @Tags Groups
// @Produce json
// @Success 200 {object} object "{success: true, data: {models: []string}}"
// @Security BearerAuth
// @Router /api/v1/group/model-list [get]
func (h *Handler) GroupModelList(c *gin.Context) {
	channels, err := dal.ListChannels(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	modelSet := make(map[string]bool)
	for _, ch := range channels {
		for _, m := range ch.Model {
			if m != "" {
				modelSet[m] = true
			}
		}
	}

	models := make([]string, 0, len(modelSet))
	for m := range modelSet {
		models = append(models, m)
	}
	sort.Strings(models)

	successJSON(c, gin.H{"models": models})
}

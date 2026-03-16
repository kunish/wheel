package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/kunish/wheel/apps/worker/internal/db/dal"
	"github.com/kunish/wheel/apps/worker/internal/types"
)

// ListTeams returns all teams.
func (h *Handler) ListTeams(c *gin.Context) {
	teams, err := dal.ListTeams(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"teams": teams})
}

// CreateTeam creates a new team.
func (h *Handler) CreateTeam(c *gin.Context) {
	var req types.TeamCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid request body")
		return
	}

	team := &types.Team{
		Name:        req.Name,
		Description: req.Description,
		MaxBudget:   req.MaxBudget,
	}

	if err := dal.CreateTeam(c.Request.Context(), h.DB, team); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successJSON(c, team)
}

// UpdateTeam updates an existing team.
func (h *Handler) UpdateTeam(c *gin.Context) {
	var req types.TeamUpdateRequest
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
	if req.Description != nil {
		data["description"] = *req.Description
	}
	if req.MaxBudget != nil {
		data["max_budget"] = *req.MaxBudget
	}

	if err := dal.UpdateTeam(c.Request.Context(), h.DB, req.ID, data); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// DeleteTeam deletes a team by ID.
func (h *Handler) DeleteTeam(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "Invalid team ID")
		return
	}

	if err := dal.DeleteTeam(c.Request.Context(), h.DB, id); err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	successNoData(c)
}

// GetTeamBudgets returns aggregated budget summaries per team.
func (h *Handler) GetTeamBudgets(c *gin.Context) {
	summaries, err := dal.GetTeamBudgetSummaries(c.Request.Context(), h.DB)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	successJSON(c, gin.H{"budgets": summaries})
}

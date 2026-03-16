package types

import (
	"time"

	"github.com/uptrace/bun"
)

// Team represents an organizational team that groups virtual keys for budget aggregation.
type Team struct {
	bun.BaseModel `bun:"table:teams"`
	ID            int       `bun:"id,pk,autoincrement" json:"id"`
	Name          string    `bun:"name,notnull"        json:"name"`
	Description   string    `bun:"description"         json:"description"`
	MaxBudget     float64   `bun:"max_budget"          json:"maxBudget"`
	CreatedAt     time.Time `bun:"created_at,nullzero,default:current_timestamp" json:"createdAt"`
	UpdatedAt     time.Time `bun:"updated_at,nullzero,default:current_timestamp" json:"updatedAt"`
}

// TeamBudgetSummary holds aggregated budget info for a team.
type TeamBudgetSummary struct {
	TeamID       int     `json:"teamId"`
	TeamName     string  `json:"teamName"`
	MaxBudget    float64 `json:"maxBudget"`
	TotalSpend   float64 `json:"totalSpend"`
	VirtualKeys  int     `json:"virtualKeys"`
	BudgetUsedPc float64 `json:"budgetUsedPercent"`
}

// TeamCreateRequest is the API request to create a team.
type TeamCreateRequest struct {
	Name        string  `json:"name" binding:"required"`
	Description string  `json:"description"`
	MaxBudget   float64 `json:"maxBudget"`
}

// TeamUpdateRequest is the API request to update a team.
type TeamUpdateRequest struct {
	ID          int      `json:"id" binding:"required"`
	Name        *string  `json:"name"`
	Description *string  `json:"description"`
	MaxBudget   *float64 `json:"maxBudget"`
}

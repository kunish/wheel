import { apiFetch } from "./client"

export interface Team {
  id: number
  name: string
  description: string
  maxBudget: number
  createdAt: string
  updatedAt: string
}

export interface TeamBudgetSummary {
  teamId: number
  teamName: string
  maxBudget: number
  totalSpend: number
  virtualKeys: number
  budgetUsedPercent: number
}

export function listTeams() {
  return apiFetch<{ success: boolean; data: { teams: Team[] } }>("/api/v1/team/list")
}

export function createTeam(data: { name: string; description?: string; maxBudget?: number }) {
  return apiFetch<{ success: boolean; data: Team }>("/api/v1/team/create", {
    method: "POST",
    body: data,
  })
}

export function updateTeam(data: {
  id: number
  name?: string
  description?: string
  maxBudget?: number
}) {
  return apiFetch<{ success: boolean }>("/api/v1/team/update", {
    method: "POST",
    body: data,
  })
}

export function deleteTeam(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/team/delete/${id}`, {
    method: "DELETE",
  })
}

export function getTeamBudgets() {
  return apiFetch<{ success: boolean; data: { budgets: TeamBudgetSummary[] } }>(
    "/api/v1/team/budgets",
  )
}

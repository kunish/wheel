import { apiFetch } from "./client"

// ── Routing Rules ──

export interface RoutingConditionItem {
  field: string
  operator: string
  value: string
}

export interface RoutingActionItem {
  type: "reject" | "route" | "rewrite"
  groupName?: string
  modelName?: string
  statusCode?: number
  message?: string
}

export interface RoutingRule {
  id: number
  name: string
  priority: number
  enabled: boolean
  conditions: RoutingConditionItem[]
  action: RoutingActionItem
}

export interface RoutingRuleInput {
  id?: number
  name: string
  priority: number
  enabled: boolean
  conditions: RoutingConditionItem[]
  action: RoutingActionItem
}

export function listRoutingRules() {
  return apiFetch<{ success: boolean; data: { rules: RoutingRule[] } }>("/api/v1/routing-rule/list")
}

export function createRoutingRule(data: Omit<RoutingRuleInput, "id">) {
  return apiFetch<{ success: boolean; data: RoutingRule }>("/api/v1/routing-rule/create", {
    method: "POST",
    body: data,
  })
}

export function updateRoutingRule(data: Partial<RoutingRuleInput> & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/routing-rule/update", {
    method: "POST",
    body: data,
  })
}

export function deleteRoutingRule(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/routing-rule/delete/${id}`, {
    method: "DELETE",
  })
}

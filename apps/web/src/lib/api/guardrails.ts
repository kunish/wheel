import { apiFetch } from "./client"

// ── Guardrail Rules ──

export interface GuardrailRule {
  id: number
  name: string
  type: "keyword" | "regex" | "length" | "pii"
  target: "input" | "output" | "both"
  action: "block" | "warn" | "redact"
  pattern: string
  maxLength: number
  enabled: boolean
}

export interface GuardrailRuleInput {
  id?: number
  name: string
  type: "keyword" | "regex" | "length" | "pii"
  target: "input" | "output" | "both"
  action: "block" | "warn" | "redact"
  pattern: string
  maxLength: number
  enabled: boolean
}

export function listGuardrailRules() {
  return apiFetch<{ success: boolean; data: { rules: GuardrailRule[] } }>("/api/v1/guardrail/list")
}

export function createGuardrailRule(data: Omit<GuardrailRuleInput, "id">) {
  return apiFetch<{ success: boolean; data: GuardrailRule }>("/api/v1/guardrail/create", {
    method: "POST",
    body: data,
  })
}

export function updateGuardrailRule(data: Partial<GuardrailRuleInput> & { id: number }) {
  return apiFetch<{ success: boolean }>("/api/v1/guardrail/update", {
    method: "POST",
    body: data,
  })
}

export function deleteGuardrailRule(id: number) {
  return apiFetch<{ success: boolean }>(`/api/v1/guardrail/delete/${id}`, {
    method: "DELETE",
  })
}

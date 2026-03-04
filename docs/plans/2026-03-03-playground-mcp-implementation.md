# Playground MCP Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add MCP support to Playground with dual modes (auto/manual), while keeping current non-MCP chat behavior unchanged.

**Architecture:** Split Playground into testable logic layers: pure MCP/tool-loop utilities, orchestration hooks, and thin UI components. MCP-enabled flows run a deterministic non-streaming tool loop in v1, then append tool results back to the conversation until final assistant text is produced.

**Tech Stack:** React 19 + TypeScript, TanStack Query, Vitest, existing `apiFetch`/`fetch` usage, Shadcn UI components.

---

### Task 1: Build MCP alias mapping utility (TDD)

**Files:**

- Create: `apps/web/src/lib/playground/mcp-alias.ts`
- Create: `apps/web/src/lib/playground/mcp-alias.test.ts`

**Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest"
import { buildToolAliasMap, type SelectedToolRef } from "./mcp-alias"

describe("buildToolAliasMap", () => {
  it("builds alias -> tool ref map and deduplicates collisions", () => {
    const selected: SelectedToolRef[] = [
      { clientId: 1, clientName: "weather", toolName: "search" },
      { clientId: 2, clientName: "weather", toolName: "search" },
    ]
    const map = buildToolAliasMap(selected)
    expect(Object.keys(map)).toContain("weather_search")
    expect(Object.keys(map)).toContain("weather_search_2")
    expect(map.weather_search.clientId).toBe(1)
    expect(map.weather_search_2.clientId).toBe(2)
  })
})
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/mcp-alias.test.ts`
Expected: FAIL with module/function missing.

**Step 3: Write minimal implementation**

```ts
export interface SelectedToolRef {
  clientId: number
  clientName: string
  toolName: string
}

export interface ToolAliasRef extends SelectedToolRef {
  alias: string
}

export function buildToolAliasMap(selected: SelectedToolRef[]): Record<string, ToolAliasRef> {
  const out: Record<string, ToolAliasRef> = {}
  const seen = new Map<string, number>()
  for (const item of selected) {
    const base = `${item.clientName}_${item.toolName}`.replace(/[\s-]+/g, "_")
    const n = (seen.get(base) ?? 0) + 1
    seen.set(base, n)
    const alias = n === 1 ? base : `${base}_${n}`
    out[alias] = { ...item, alias }
  }
  return out
}
```

**Step 4: Run test to verify it passes**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/mcp-alias.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/lib/playground/mcp-alias.ts apps/web/src/lib/playground/mcp-alias.test.ts
git commit -m "test(playground): add MCP tool alias map utility"
```

### Task 2: Build tool-loop pure helpers (TDD)

**Files:**

- Create: `apps/web/src/lib/playground/tool-loop.ts`
- Create: `apps/web/src/lib/playground/tool-loop.test.ts`

**Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest"
import { extractToolCalls, makeToolMessage, shouldStopLoop } from "./tool-loop"

describe("tool-loop", () => {
  it("extracts tool calls from assistant message", () => {
    const calls = extractToolCalls({
      choices: [
        { message: { tool_calls: [{ id: "c1", function: { name: "a", arguments: "{}" } }] } },
      ],
    })
    expect(calls).toHaveLength(1)
    expect(calls[0].id).toBe("c1")
  })

  it("builds tool role message with matching tool_call_id", () => {
    const msg = makeToolMessage("c1", { ok: true })
    expect(msg.role).toBe("tool")
    expect(msg.tool_call_id).toBe("c1")
  })

  it("stops when rounds exceed max", () => {
    expect(shouldStopLoop({ round: 7, maxRounds: 6, pendingCalls: 1 })).toBe(true)
  })
})
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/tool-loop.test.ts`
Expected: FAIL with module/function missing.

**Step 3: Write minimal implementation**

```ts
export interface ToolCall {
  id: string
  name: string
  argumentsText: string
}

export function extractToolCalls(resp: any): ToolCall[] {
  const raw = resp?.choices?.[0]?.message?.tool_calls ?? []
  return raw.map((x: any) => ({
    id: x.id,
    name: x.function?.name ?? "",
    argumentsText: x.function?.arguments ?? "{}",
  }))
}

export function makeToolMessage(toolCallId: string, payload: unknown) {
  return { role: "tool", tool_call_id: toolCallId, content: JSON.stringify(payload) }
}

export function shouldStopLoop(input: { round: number; maxRounds: number; pendingCalls: number }) {
  return input.round > input.maxRounds || input.pendingCalls === 0
}
```

**Step 4: Run test to verify it passes**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/tool-loop.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/lib/playground/tool-loop.ts apps/web/src/lib/playground/tool-loop.test.ts
git commit -m "test(playground): add MCP tool loop helpers"
```

### Task 3: Add Playground API helpers (TDD-first for body builders)

**Files:**

- Create: `apps/web/src/lib/playground/request-builders.ts`
- Create: `apps/web/src/lib/playground/request-builders.test.ts`
- Create: `apps/web/src/lib/api/playground.ts`
- Modify: `apps/web/src/lib/api/index.ts`

**Step 1: Write the failing test for payload builders**

```ts
import { describe, expect, it } from "vitest"
import { buildChatPayload, buildMcpToolExecutePayload } from "../playground/request-builders"

it("builds chat payload with tools when MCP enabled", () => {
  const body = buildChatPayload({
    model: "gpt-4o",
    messages: [],
    mcpTools: [{ type: "function", function: { name: "x" } }],
  })
  expect(body.tools).toHaveLength(1)
})

it("builds mcp execute payload", () => {
  const body = buildMcpToolExecutePayload(1, "weather_search", { q: "Tokyo" })
  expect(body.clientId).toBe(1)
})
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/request-builders.test.ts`
Expected: FAIL with missing builders.

**Step 3: Implement builders and API functions**

```ts
// src/lib/api/playground.ts
export async function createChatCompletion(init: {
  apiKey: string
  body: Record<string, unknown>
  signal?: AbortSignal
}) {
  /* POST /v1/chat/completions */
}

export async function executeMcpTool(init: {
  apiKey: string
  clientId: number
  toolName: string
  argumentsObj: Record<string, unknown>
  signal?: AbortSignal
}) {
  /* POST /v1/mcp/tool/execute */
}
```

**Step 4: Run tests to verify they pass**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/request-builders.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/lib/playground/request-builders.ts apps/web/src/lib/playground/request-builders.test.ts apps/web/src/lib/api/playground.ts apps/web/src/lib/api/index.ts
git commit -m "feat(playground): add MCP-aware request builders and API helpers"
```

### Task 4: Add MCP selection hook for Playground (TDD on pure selector)

**Files:**

- Create: `apps/web/src/lib/playground/tool-selection.ts`
- Create: `apps/web/src/lib/playground/tool-selection.test.ts`
- Create: `apps/web/src/hooks/use-playground-mcp.ts`

**Step 1: Write the failing test**

```ts
import { describe, expect, it } from "vitest"
import { selectMcpTools } from "./tool-selection"

it("filters to connected clients and selected tools", () => {
  const result = selectMcpTools(/* fixture clients + selected ids */)
  expect(result.aliasMap).toBeTruthy()
  expect(result.toolsPayload.length).toBeGreaterThan(0)
})
```

**Step 2: Run test to verify it fails**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/tool-selection.test.ts`
Expected: FAIL.

**Step 3: Implement selector + hook**

```ts
export function usePlaygroundMcp() {
  // query mcp-clients
  // derive selected tools
  // return aliasMap + tools payload + UI options
}
```

**Step 4: Run test to verify it passes**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/tool-selection.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/lib/playground/tool-selection.ts apps/web/src/lib/playground/tool-selection.test.ts apps/web/src/hooks/use-playground-mcp.ts
git commit -m "feat(playground): add MCP tool selection model"
```

### Task 5: Implement auto/manual loop runner (TDD)

**Files:**

- Create: `apps/web/src/lib/playground/chat-runner.ts`
- Create: `apps/web/src/lib/playground/chat-runner.test.ts`

**Step 1: Write failing tests for both modes**

```ts
describe("chat-runner auto mode", () => {
  it("executes tool calls then requests final assistant answer", async () => {
    // mock sequence: completion with tool_calls -> execute -> completion with text
  })
})

describe("chat-runner manual mode", () => {
  it("pauses after tool_calls until continue is invoked", async () => {
    // assert paused state and no second completion call
  })
})
```

**Step 2: Run tests to verify they fail**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/chat-runner.test.ts`
Expected: FAIL.

**Step 3: Implement minimal runner**

```ts
export async function runPlaygroundChatLoop(input: RunLoopInput): Promise<RunLoopResult> {
  // handles request -> tool_calls -> execute tools -> continue
  // enforces maxRounds/maxToolCalls
}
```

**Step 4: Run tests to verify they pass**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/chat-runner.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/lib/playground/chat-runner.ts apps/web/src/lib/playground/chat-runner.test.ts
git commit -m "feat(playground): add dual-mode MCP loop runner"
```

### Task 6: Add page-level orchestration hook and UI components

**Files:**

- Create: `apps/web/src/hooks/use-playground-chat.ts`
- Create: `apps/web/src/components/playground/mcp-panel.tsx`
- Create: `apps/web/src/components/playground/tool-call-timeline.tsx`

**Step 1: Write a failing test for timeline status formatting**

```ts
import { expect, it } from "vitest"
import { formatToolCallStatus } from "@/components/playground/tool-call-timeline"

it("formats error status with message", () => {
  expect(formatToolCallStatus({ status: "error", error: "boom" })).toContain("boom")
})
```

**Step 2: Run targeted test**

Run: `pnpm --filter @wheel/web exec vitest run src/components/playground/tool-call-timeline.test.ts`
Expected: FAIL.

**Step 3: Implement minimal components + hook wiring**

```tsx
// mcp-panel.tsx: enable switch, mode select, tool multi-select
// tool-call-timeline.tsx: pending/running/ok/error list
// use-playground-chat.ts: wraps runPlaygroundChatLoop and manages abort/stat/error
```

**Step 4: Run targeted test**

Run: `pnpm --filter @wheel/web exec vitest run src/components/playground/tool-call-timeline.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/hooks/use-playground-chat.ts apps/web/src/components/playground/mcp-panel.tsx apps/web/src/components/playground/tool-call-timeline.tsx apps/web/src/components/playground/tool-call-timeline.test.ts
git commit -m "feat(playground): add MCP controls and tool timeline UI"
```

### Task 7: Integrate optimized Playground page

**Files:**

- Modify: `apps/web/src/pages/playground.tsx`

**Step 1: Write integration-oriented failing test for command state guard**

```ts
import { expect, it } from "vitest"
import { canSendPlaygroundRequest } from "@/lib/playground/request-builders"

it("disables send when MCP manual mode is paused", () => {
  expect(canSendPlaygroundRequest({ isLoading: false, isPaused: true })).toBe(false)
})
```

**Step 2: Run test to verify failure**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/request-builders.test.ts`
Expected: FAIL for missing rule.

**Step 3: Refactor `playground.tsx` to wire new hooks/components**

```tsx
// Replace inline fetch loop with usePlaygroundChat + usePlaygroundMcp
// Render MCPPanel in parameter section
// Render ToolCallTimeline under response card
// Keep non-MCP path and current keyboard shortcuts
```

**Step 4: Run related tests**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/*.test.ts src/components/playground/*.test.ts`
Expected: PASS.

**Step 5: Commit**

```bash
git add apps/web/src/pages/playground.tsx
git commit -m "refactor(playground): integrate MCP dual-mode orchestration"
```

### Task 8: i18n + full verification

**Files:**

- Modify: `apps/web/src/i18n/locales/en/playground.json`
- Modify: `apps/web/src/i18n/locales/zh-CN/playground.json`

**Step 1: Add failing key-coverage check (simple static test)**

```ts
import en from "@/i18n/locales/en/playground.json"
import zh from "@/i18n/locales/zh-CN/playground.json"
import { expect, it } from "vitest"

it("zh playground locale has all en keys", () => {
  expect(Object.keys(zh).sort()).toEqual(Object.keys(en).sort())
})
```

**Step 2: Run test to verify failure**

Run: `pnpm --filter @wheel/web exec vitest run src/lib/playground/i18n-keys.test.ts`
Expected: FAIL before adding new keys.

**Step 3: Add new MCP-related locale keys in both files**

```json
{
  "mcp": {
    "enabled": "Enable MCP",
    "mode": "Mode",
    "modeAuto": "Auto",
    "modeManual": "Manual",
    "toolCalls": "Tool Calls",
    "executeAll": "Execute All",
    "continue": "Continue"
  }
}
```

**Step 4: Run full verification**

Run:

- `pnpm --filter @wheel/web run lint .`
- `pnpm --filter @wheel/web run test`
- `pnpm --filter @wheel/web run build`

Expected: all PASS.

**Step 5: Commit**

```bash
git add apps/web/src/i18n/locales/en/playground.json apps/web/src/i18n/locales/zh-CN/playground.json apps/web/src/lib/playground/i18n-keys.test.ts
git commit -m "feat(playground): finalize MCP UX copy and regression checks"
```

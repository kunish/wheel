# Playground MCP Design

## Goal

Upgrade the Playground page to support MCP tool calls with two interaction modes (auto and manual), while preserving the current plain chat experience when MCP is disabled.

## Scope

- Enhance `apps/web/src/pages/playground.tsx` with MCP-aware orchestration.
- Reuse existing MCP admin APIs and tool execution endpoint:
  - `GET /api/v1/mcp/client/list`
  - `POST /v1/mcp/tool/execute`
  - `POST /v1/chat/completions`
- Add UI for MCP mode switching, tool selection, and tool-call timeline.
- Keep current route structure and auth model.

## Non-Goals

- No backend MCP protocol redesign.
- No new MCP persistence model.
- No streaming tool-call assembly in the first MCP-enabled version.

## Architecture

### 1) Page orchestration

Playground remains the route entry, but orchestration is split into focused logic modules:

- Chat request lifecycle (messages, stats, abort, response state)
- MCP capability loading (clients and tools)
- Tool loop runner (auto/manual execution loop)
- UI-only rendering blocks

### 2) MCP tool mapping layer

The page builds a stable alias map for tools:

- Public alias exposed to LLM: `clientName_toolName`
- Internal mapping: `alias -> { clientId, originalToolName, clientName }`

This avoids collisions across clients and gives deterministic lookup for tool execution.

### 3) Mode model

- Auto mode: run tool-call loop until final assistant answer.
- Manual mode: pause on `tool_calls`, let users inspect/execute, then continue.

## Data Flow

### Base request assembly

1. Build conversation messages (`system`, `user`, plus later `assistant/tool`).
2. If MCP is enabled, inject `tools` array into `/v1/chat/completions` request.
3. Send request with selected API key and current sampling parameters.

### Auto mode loop

1. Request completion (MCP-enabled requests default to non-streaming in v1).
2. If no `tool_calls`, finish and render assistant answer.
3. If `tool_calls` exists:
   - Execute each call via `POST /v1/mcp/tool/execute`
   - Convert result to `role=tool` message using `tool_call_id`
   - Append back to conversation
4. Repeat until final answer or loop limit.

### Manual mode loop

1. Request completion.
2. If `tool_calls` exists, move to paused state.
3. Show pending calls and argument payloads.
4. User executes one/all calls.
5. Append tool messages.
6. User clicks continue to request next assistant turn.

## State Model

Core states:

- `idle`
- `requesting_llm`
- `waiting_tool_calls`
- `executing_tools`
- `completed`
- `aborted`
- `error`

Transitions:

- `idle -> requesting_llm`
- `requesting_llm -> waiting_tool_calls` (if tool calls exist)
- `waiting_tool_calls -> executing_tools` (auto or manual trigger)
- `executing_tools -> requesting_llm`
- Any active state can go to `aborted` or `error`.

## UI Plan

### MCP panel (inside Playground parameters)

- Enable MCP switch
- Mode switch: Auto / Manual
- Tool selector grouped by client
- Lightweight helper text for mode behavior

### Tool Calls timeline

- Show each call item with:
  - tool alias and resolved client
  - arguments (collapsed JSON)
  - duration
  - status (`pending`, `running`, `ok`, `error`)
  - error message if failed

### Manual controls

- Execute one
- Execute all
- Continue conversation

## Error Handling

- Missing alias mapping -> mark call failed and append tool error message.
- `/v1/mcp/tool/execute` failure -> append tool error message and keep flow alive.
- Loop guard:
  - max rounds per send
  - max tool calls per round
- Abort support:
  - cancel active LLM request
  - stop pending auto loop
- Preserve conversation context after failure for retry.

## Testing Strategy

### Unit tests (pure logic)

- alias mapping generation and lookup
- tool-call extraction and serialization
- loop termination guards
- manual mode pause/continue transitions

### Integration-style tests (fetch mocked)

- auto mode: `tool_calls -> execute -> final assistant`
- manual mode: pause, execute, continue
- tool execution error path
- MCP disabled behavior parity with existing chat flow

### Regression checks

- `pnpm --filter @wheel/web run lint .`
- `pnpm --filter @wheel/web run test`
- `pnpm --filter @wheel/web run build`

## Acceptance Criteria

- MCP auto and manual mode both complete valid end-to-end tool loops.
- Non-MCP usage remains unchanged.
- Errors and abort are visible and recoverable.
- Web lint/test/build pass.

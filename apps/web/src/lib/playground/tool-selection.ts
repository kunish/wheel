import type { SelectedToolRef, ToolAliasRef } from "./mcp-alias"
import type { McpToolDef } from "./request-builders"
import type { MCPClientRecord } from "@/lib/api"
import { buildToolAliasMap } from "./mcp-alias"

export interface SelectableToolRef extends SelectedToolRef {
  key: string
  description?: string
}

function toToolKey(clientId: number, toolName: string): string {
  return `${clientId}:${toolName}`
}

export function buildSelectableTools(clients: MCPClientRecord[]): SelectableToolRef[] {
  const out: SelectableToolRef[] = []

  for (const client of clients) {
    if (!client.enabled || client.state !== "connected") continue

    for (const tool of client.tools ?? []) {
      out.push({
        key: toToolKey(client.id, tool.name),
        clientId: client.id,
        clientName: client.name,
        toolName: tool.name,
        description: tool.description,
      })
    }
  }

  return out.sort((a, b) => {
    const byClient = a.clientName.localeCompare(b.clientName)
    if (byClient !== 0) return byClient
    return a.toolName.localeCompare(b.toolName)
  })
}

export function pickSelectedToolKeys(
  available: SelectableToolRef[],
  previous: string[],
  options?: { fallbackToAll?: boolean },
): string[] {
  const availableSet = new Set(available.map((x) => x.key))
  const valid = previous.filter((x) => availableSet.has(x))
  if (valid.length > 0) return valid.sort()
  if (options?.fallbackToAll === false) return []
  return available.map((x) => x.key).sort()
}

export function toSelectedToolRefs(
  available: SelectableToolRef[],
  selectedKeys: string[],
): SelectedToolRef[] {
  const selectedSet = new Set(selectedKeys)
  return available
    .filter((x) => selectedSet.has(x.key))
    .map(({ clientId, clientName, toolName }) => ({ clientId, clientName, toolName }))
}

export function buildMcpToolsForChat(selected: Array<SelectedToolRef & { description?: string }>): {
  tools: McpToolDef[]
  aliasMap: Record<string, ToolAliasRef>
} {
  const refs: SelectedToolRef[] = selected.map(({ clientId, clientName, toolName }) => ({
    clientId,
    clientName,
    toolName,
  }))
  const aliasMap = buildToolAliasMap(refs)

  const byKey = new Map(selected.map((x) => [toToolKey(x.clientId, x.toolName), x]))
  const tools: McpToolDef[] = Object.values(aliasMap).map((aliasRef) => {
    const source = byKey.get(toToolKey(aliasRef.clientId, aliasRef.toolName))
    const description = source?.description?.trim()
    return {
      type: "function",
      function: {
        name: aliasRef.alias,
        description: description
          ? `[${aliasRef.clientName}] ${description}`
          : `[${aliasRef.clientName}] ${aliasRef.toolName}`,
        parameters: {
          type: "object",
          additionalProperties: true,
        },
      },
    }
  })

  return { tools, aliasMap }
}

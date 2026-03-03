import type { ToolAliasRef } from "@/lib/playground/mcp-alias"
import type { McpToolDef } from "@/lib/playground/request-builders"
import type { SelectableToolRef } from "@/lib/playground/tool-selection"
import { useQuery } from "@tanstack/react-query"
import { useMemo, useState } from "react"
import { listMCPClients } from "@/lib/api"
import {
  buildMcpToolsForChat,
  buildSelectableTools,
  pickSelectedToolKeys,
  toSelectedToolRefs,
} from "@/lib/playground/tool-selection"

export type PlaygroundMcpMode = "auto" | "manual"

export interface UsePlaygroundMcpResult {
  enabled: boolean
  mode: PlaygroundMcpMode
  loading: boolean
  tools: SelectableToolRef[]
  selectedKeys: string[]
  selectedCount: number
  mcpTools: McpToolDef[]
  aliasMap: Record<string, ToolAliasRef>
  setEnabled: (next: boolean) => void
  setMode: (next: PlaygroundMcpMode) => void
  setSelectedKeys: (next: string[]) => void
  toggleTool: (key: string) => void
  selectAll: () => void
  clearAll: () => void
}

export function usePlaygroundMcp(): UsePlaygroundMcpResult {
  const [enabled, setEnabled] = useState(false)
  const [mode, setMode] = useState<PlaygroundMcpMode>("auto")
  const [selectedKeys, setSelectedKeys] = useState<string[]>([])

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-clients"],
    queryFn: listMCPClients,
  })

  const tools = useMemo(() => {
    return buildSelectableTools(data?.data?.clients ?? [])
  }, [data?.data?.clients])

  const effectiveSelectedKeys = useMemo(
    () => pickSelectedToolKeys(tools, selectedKeys),
    [tools, selectedKeys],
  )

  const selected = useMemo(() => {
    return toSelectedToolRefs(tools, effectiveSelectedKeys)
  }, [tools, effectiveSelectedKeys])

  const mcpDefs = useMemo(() => {
    if (!enabled || selected.length === 0) {
      return {
        tools: [] as McpToolDef[],
        aliasMap: {} as Record<string, ToolAliasRef>,
      }
    }
    const selectedWithMeta = selected.map((item) => {
      const raw = tools.find((x) => x.clientId === item.clientId && x.toolName === item.toolName)
      return { ...item, description: raw?.description }
    })
    return buildMcpToolsForChat(selectedWithMeta)
  }, [enabled, selected, tools])

  return {
    enabled,
    mode,
    loading: isLoading,
    tools,
    selectedKeys: effectiveSelectedKeys,
    selectedCount: selected.length,
    mcpTools: mcpDefs.tools,
    aliasMap: mcpDefs.aliasMap,
    setEnabled,
    setMode,
    setSelectedKeys,
    toggleTool: (key: string) => {
      setSelectedKeys((prev) =>
        prev.includes(key) ? prev.filter((x) => x !== key) : [...prev, key].sort(),
      )
    },
    selectAll: () => setSelectedKeys(tools.map((x) => x.key).sort()),
    clearAll: () => setSelectedKeys([]),
  }
}

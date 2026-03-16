import type { HeaderContext } from "@tanstack/react-table"
import type { TFunction } from "i18next"
import type { ReactNode } from "react"
import { createColumnHelper } from "@tanstack/react-table"
import { ArrowDown, ArrowUp, ArrowUpDown, Eye, Layers, Loader2 } from "lucide-react"
import { Link } from "react-router"
import { ModelBadge } from "@/components/model-badge"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Tooltip, TooltipContent, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"
import { deriveLastMessagePreview } from "./preview"

export interface LogEntry {
  id: number
  time: number
  requestModelName: string
  actualModelName: string
  channelId: number
  channelName: string
  inputTokens: number
  outputTokens: number
  cacheReadTokens: number
  cacheCreationTokens: number
  ftut: number
  useTime: number
  error: string
  cost?: number
  totalAttempts: number
  _streaming?: boolean
  _streamId?: string
  _startedAt?: number
  _inputPrice?: number
  _outputPrice?: number
  _estimatedInputTokens?: number
  _requestContent?: string
  lastMessagePreview?: string
}

function formatDuration(ms: number): string {
  if (ms < 1000) return `${ms}ms`
  return `${(ms / 1000).toFixed(2)}s`
}

function formatCost(cost: number | undefined): string {
  if (!cost || cost === 0) return "$0"
  if (cost < 0.000001) return `$${cost.toExponential(1)}`
  if (cost < 0.01) return `$${cost.toFixed(6)}`
  return `$${cost.toFixed(4)}`
}

function formatTime(ts: number): string {
  const d = new Date(ts * 1000)
  const month = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  const hours = String(d.getHours()).padStart(2, "0")
  const minutes = String(d.getMinutes()).padStart(2, "0")
  const seconds = String(d.getSeconds()).padStart(2, "0")
  return `${month}/${day} ${hours}:${minutes}:${seconds}`
}

// Sortable header content with three-state icon (rendered inside <TableHead> by page.tsx)
function SortableHeader({
  column,
  children,
}: {
  column: HeaderContext<LogEntry, unknown>["column"]
  children: ReactNode
}) {
  const sorted = column.getIsSorted()
  return (
    <button
      type="button"
      className="inline-flex cursor-pointer items-center gap-1 select-none"
      onClick={column.getToggleSortingHandler()}
    >
      {children}
      {sorted === "asc" ? (
        <ArrowUp className="h-3 w-3" />
      ) : sorted === "desc" ? (
        <ArrowDown className="h-3 w-3" />
      ) : (
        <ArrowUpDown className="text-muted-foreground/50 h-3 w-3" />
      )}
    </button>
  )
}

// Groupable header — click to toggle grouping on this column
function GroupableHeader({
  column,
  children,
}: {
  column: HeaderContext<LogEntry, unknown>["column"]
  children: ReactNode
}) {
  const isGrouped = column.getIsGrouped()
  return (
    <button
      type="button"
      className="inline-flex cursor-pointer items-center gap-1 select-none"
      onClick={column.getToggleGroupingHandler()}
    >
      {children}
      <Layers className={`h-3 w-3 ${isGrouped ? "text-foreground" : "text-muted-foreground/50"}`} />
    </button>
  )
}

const columnHelper = createColumnHelper<LogEntry>()

export function createLogColumns(onViewDetail: (id: number) => void, t: TFunction) {
  return [
    columnHelper.accessor("time", {
      header: t("columns.time"),
      enableSorting: false,
      cell: (info) => (
        <span className="font-mono text-xs whitespace-nowrap">{formatTime(info.getValue())}</span>
      ),
    }),
    columnHelper.accessor("requestModelName", {
      header: ({ column }) => (
        <GroupableHeader column={column}>{t("columns.model")}</GroupableHeader>
      ),
      enableSorting: false,
      enableGrouping: true,
      cell: (info) => {
        const row = info.row.original
        return (
          <div className="flex flex-col gap-0.5">
            <Tooltip>
              <TooltipTrigger asChild>
                {row.channelId ? (
                  <Link
                    to={`/model?highlight=${row.channelId}`}
                    onClick={(e) => e.stopPropagation()}
                    className="hover:underline"
                  >
                    <ModelBadge modelId={row.requestModelName} />
                  </Link>
                ) : (
                  <ModelBadge modelId={row.requestModelName} />
                )}
              </TooltipTrigger>
              <TooltipContent>
                <p className="font-mono text-xs">{row.requestModelName}</p>
              </TooltipContent>
            </Tooltip>
            {row.actualModelName && row.actualModelName !== row.requestModelName && (
              <span className="text-muted-foreground max-w-[150px] truncate text-[10px]">
                {row.actualModelName}
              </span>
            )}
          </div>
        )
      },
    }),
    columnHelper.display({
      id: "preview",
      header: t("columns.preview"),
      cell: (info) => {
        const row = info.row.original
        const preview = row.lastMessagePreview || deriveLastMessagePreview(row._requestContent)
        if (!preview) {
          return <span className="text-muted-foreground text-xs">—</span>
        }
        return (
          <Tooltip>
            <TooltipTrigger asChild>
              <span className="block max-w-[320px] truncate text-xs">{preview}</span>
            </TooltipTrigger>
            <TooltipContent className="max-w-lg">
              <p className="text-xs break-all whitespace-pre-wrap">{preview}</p>
            </TooltipContent>
          </Tooltip>
        )
      },
    }),
    columnHelper.accessor("channelName", {
      header: ({ column }) => (
        <GroupableHeader column={column}>{t("columns.channel")}</GroupableHeader>
      ),
      enableSorting: false,
      enableGrouping: true,
      cell: (info) => {
        const row = info.row.original
        return (
          <div className="flex items-center gap-1 text-xs">
            {row.channelId ? (
              <Link
                to={`/model?highlight=${row.channelId}`}
                onClick={(e) => e.stopPropagation()}
                className="hover:underline"
              >
                {row.channelName || "—"}
              </Link>
            ) : (
              <span>{row.channelName || "—"}</span>
            )}
            {row.totalAttempts > 1 && (
              <Tooltip>
                <TooltipTrigger asChild>
                  <Badge variant="outline" className="px-1 py-0 text-[10px]">
                    R{row.totalAttempts}
                  </Badge>
                </TooltipTrigger>
                <TooltipContent>
                  {t("columns.attempts", { count: row.totalAttempts })}
                </TooltipContent>
              </Tooltip>
            )}
          </div>
        )
      },
    }),
    columnHelper.accessor("inputTokens", {
      header: ({ column }) => <SortableHeader column={column}>{t("columns.input")}</SortableHeader>,
      enableSorting: true,
      cell: (info) => {
        const row = info.row.original
        const cacheRead = row.cacheReadTokens || 0
        return (
          <div className={cn("text-right font-mono text-xs", row._streaming && "opacity-50")}>
            <span>{info.getValue().toLocaleString()}</span>
            {cacheRead > 0 && (
              <span className="text-muted-foreground ml-1 text-[10px]">
                ({cacheRead.toLocaleString()} {t("columns.cached")})
              </span>
            )}
          </div>
        )
      },
      meta: { className: "text-right" },
    }),
    columnHelper.accessor("outputTokens", {
      header: ({ column }) => (
        <SortableHeader column={column}>{t("columns.output")}</SortableHeader>
      ),
      enableSorting: true,
      cell: (info) => (
        <span
          className={cn(
            "text-right font-mono text-xs",
            info.row.original._streaming && "opacity-50",
          )}
        >
          {info.getValue().toLocaleString()}
        </span>
      ),
      meta: { className: "text-right" },
    }),
    columnHelper.accessor("ftut", {
      header: ({ column }) => <SortableHeader column={column}>{t("columns.ttft")}</SortableHeader>,
      enableSorting: true,
      cell: (info) => (
        <span className="text-muted-foreground text-right font-mono text-xs">
          {info.getValue() > 0 ? formatDuration(info.getValue()) : "—"}
        </span>
      ),
      meta: { className: "text-right" },
    }),
    columnHelper.accessor("useTime", {
      header: ({ column }) => (
        <SortableHeader column={column}>{t("columns.latency")}</SortableHeader>
      ),
      enableSorting: true,
      cell: (info) => (
        <span className="text-right font-mono text-xs">{formatDuration(info.getValue())}</span>
      ),
      meta: { className: "text-right" },
    }),
    columnHelper.accessor("cost", {
      header: ({ column }) => <SortableHeader column={column}>{t("columns.cost")}</SortableHeader>,
      enableSorting: true,
      cell: (info) => (
        <span
          className={cn(
            "text-right font-mono text-xs",
            info.row.original._streaming && "opacity-50",
          )}
        >
          {formatCost(info.getValue())}
        </span>
      ),
      meta: { className: "text-right" },
    }),
    columnHelper.display({
      id: "status",
      header: t("columns.status"),
      cell: (info) => {
        const row = info.row.original
        if (row._streaming) {
          return (
            <Badge variant="outline" className="h-6 w-[88px] animate-pulse justify-center gap-1">
              <Loader2 className="h-2.5 w-2.5 animate-spin" />
              {t("columns.streaming")}
            </Badge>
          )
        }
        return row.error ? (
          <Tooltip>
            <TooltipTrigger asChild>
              <Badge variant="destructive" className="h-6 w-[88px] justify-center">
                {t("columns.error")}
              </Badge>
            </TooltipTrigger>
            <TooltipContent className="max-w-xs">
              <p className="text-xs break-all whitespace-pre-wrap">
                {row.error.length > 200 ? `${row.error.slice(0, 200)}...` : row.error}
              </p>
            </TooltipContent>
          </Tooltip>
        ) : (
          <Badge variant="default" className="h-6 w-[88px] justify-center">
            {t("columns.ok")}
          </Badge>
        )
      },
    }),
    columnHelper.display({
      id: "actions",
      header: "",
      cell: (info) => {
        const row = info.row.original
        if (row._streaming) {
          return <div className="h-8 w-8" aria-hidden="true" />
        }
        return (
          <Button
            variant="ghost"
            size="icon"
            aria-label="View log detail"
            onClick={(e) => {
              e.stopPropagation()
              onViewDetail(row.id)
            }}
          >
            <Eye className="h-4 w-4" />
          </Button>
        )
      },
      meta: { className: "w-10" },
    }),
  ]
}

// Re-export formatters needed by detail panel
export { formatCost, formatDuration, formatTime }

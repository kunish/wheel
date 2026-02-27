import type { ExpandedState, GroupingState, SortingState } from "@tanstack/react-table"
import type { LogEntry } from "./columns"
import {
  flexRender,
  getCoreRowModel,
  getExpandedRowModel,
  getGroupedRowModel,
  getSortedRowModel,
  useReactTable,
} from "@tanstack/react-table"
import { AlertCircle, ChevronLeft, ChevronRight, FileText, RefreshCw } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useCallback, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { TooltipProvider } from "@/components/ui/tooltip"
import { createLogColumns } from "./columns"
import { useLogQueryContext } from "./log-query-context"

// Hoist row model factories outside the component to maintain stable references
const coreRowModel = getCoreRowModel<LogEntry>()
const sortedRowModel = getSortedRowModel<LogEntry>()
const groupedRowModel = getGroupedRowModel<LogEntry>()
const expandedRowModel = getExpandedRowModel<LogEntry>()

// Hoisted constant arrays to avoid re-creation on every render
const SKELETON_ROWS_8 = Array.from({ length: 8 })
const SKELETON_ROWS_10 = Array.from({ length: 10 })

export function LogTable() {
  const { t } = useTranslation("logs")
  const {
    logs,
    pageSize,
    isLoading,
    isFetching,
    isError,
    hasFilters,
    refetch,
    setDetailId,
    setDetailStreamId,
    navigate,
    pathname,
    setKeywordInput,
  } = useLogQueryContext()
  const [sorting, setSorting] = useState<SortingState>([])
  const [grouping, setGrouping] = useState<GroupingState>([])
  const [expanded, setExpanded] = useState<ExpandedState>(true)

  const onRowClick = useCallback(
    (log: LogEntry) => {
      if (log._streaming && log._streamId) {
        setDetailStreamId(log._streamId)
      } else {
        setDetailStreamId(null)
        setDetailId(log.id)
      }
    },
    [setDetailId, setDetailStreamId],
  )

  const onClearFilters = useCallback(() => {
    navigate(pathname, { replace: true })
    setKeywordInput("")
  }, [navigate, pathname, setKeywordInput])

  const onViewDetail = setDetailId

  const columns = useMemo(() => createLogColumns(onViewDetail, t), [t, onViewDetail])

  const table = useReactTable({
    data: logs,
    columns,
    state: { sorting, grouping, expanded },
    onSortingChange: setSorting,
    onGroupingChange: setGrouping,
    onExpandedChange: setExpanded,
    enableSortingRemoval: true,
    getCoreRowModel: coreRowModel,
    getSortedRowModel: sortedRowModel,
    getGroupedRowModel: groupedRowModel,
    getExpandedRowModel: expandedRowModel,
  })

  if (isError) {
    return (
      <Card className="flex flex-col items-center justify-center gap-3 py-12">
        <AlertCircle className="text-destructive h-8 w-8" />
        <p className="text-muted-foreground text-sm">{t("loadError")}</p>
        <Button variant="outline" size="sm" className="gap-1.5" onClick={() => refetch()}>
          <RefreshCw className="h-3.5 w-3.5" />
          {t("actions.retry", { ns: "common" })}
        </Button>
      </Card>
    )
  }

  if (isLoading) {
    return <LogTableSkeleton rows={pageSize > 20 ? 10 : 8} />
  }

  return (
    <div
      className={`min-h-0 flex-1 overflow-auto transition-opacity duration-150 ${isFetching ? "pointer-events-none opacity-50" : ""}`}
    >
      <TooltipProvider delayDuration={300}>
        <table className="w-full caption-bottom text-sm">
          <thead className="bg-muted sticky top-0 z-10 [&_tr]:border-b-2">
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead
                    key={header.id}
                    className={(header.column.columnDef.meta as { className?: string })?.className}
                  >
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </thead>
          <TableBody>
            <AnimatePresence initial={false}>
              {table.getRowModel().rows.map((row) => {
                if (row.getIsGrouped()) {
                  return (
                    <motion.tr
                      key={row.id}
                      initial={{ opacity: 0, y: -10 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -10 }}
                      transition={{ duration: 0.2 }}
                      className="bg-muted/30"
                    >
                      <TableCell colSpan={columns.length}>
                        <button
                          className="flex items-center gap-1.5 text-sm font-medium"
                          onClick={row.getToggleExpandedHandler()}
                        >
                          <ChevronRight
                            className={`h-4 w-4 transition-transform ${row.getIsExpanded() ? "rotate-90" : ""}`}
                          />
                          {String(row.groupingValue)}
                          <Badge variant="secondary" className="text-xs">
                            {row.subRows.length}
                          </Badge>
                        </button>
                      </TableCell>
                    </motion.tr>
                  )
                }
                const log = row.original
                return (
                  <motion.tr
                    key={row.id}
                    initial={{ opacity: 0, y: -10 }}
                    animate={{ opacity: 1, y: 0 }}
                    exit={{ opacity: 0, y: -10 }}
                    transition={{ duration: 0.2 }}
                    className={`hover:bg-muted/50 cursor-pointer border-b ${
                      log._streaming
                        ? "bg-muted/20"
                        : log.error
                          ? "border-l-destructive bg-destructive/5 border-l-2"
                          : ""
                    }`}
                    onClick={() => onRowClick(log)}
                  >
                    {row.getVisibleCells().map((cell) => (
                      <TableCell
                        key={cell.id}
                        className={
                          (cell.column.columnDef.meta as { className?: string })?.className
                        }
                      >
                        {flexRender(cell.column.columnDef.cell, cell.getContext())}
                      </TableCell>
                    ))}
                  </motion.tr>
                )
              })}
            </AnimatePresence>
            {logs.length === 0 && !isLoading && (
              <TableRow>
                <TableCell colSpan={columns.length} className="py-12 text-center">
                  <div className="flex flex-col items-center gap-2">
                    <FileText className="text-muted-foreground/30 h-10 w-10" />
                    <p className="text-muted-foreground">
                      {hasFilters ? t("empty.noMatch") : t("empty.noLogs")}
                    </p>
                    {hasFilters && (
                      <Button variant="outline" size="sm" className="mt-1" onClick={onClearFilters}>
                        {t("empty.clearFilters")}
                      </Button>
                    )}
                  </div>
                </TableCell>
              </TableRow>
            )}
          </TableBody>
        </table>
      </TooltipProvider>
    </div>
  )
}

interface PaginationControlsProps {
  page: number
  pageSize: number
  totalPages: number
  updateFilter: (updates: Record<string, string | number | undefined | null>) => void
}

export function PaginationControls({
  page,
  pageSize,
  totalPages,
  updateFilter,
}: PaginationControlsProps) {
  return (
    <div className="flex items-center gap-2">
      <Select value={String(pageSize)} onValueChange={(v) => updateFilter({ size: v })}>
        <SelectTrigger className="h-8 w-20 text-xs">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value="50">50</SelectItem>
          <SelectItem value="100">100</SelectItem>
          <SelectItem value="200">200</SelectItem>
          <SelectItem value="500">500</SelectItem>
        </SelectContent>
      </Select>
      <span className="text-muted-foreground text-sm tabular-nums">
        {page}/{totalPages || 1}
      </span>
      <Button
        variant="outline"
        size="icon-xs"
        disabled={page <= 1}
        onClick={() => updateFilter({ page: page - 1 })}
      >
        <ChevronLeft className="h-3.5 w-3.5" />
      </Button>
      <Button
        variant="outline"
        size="icon-xs"
        disabled={page >= totalPages}
        onClick={() => updateFilter({ page: page + 1 })}
      >
        <ChevronRight className="h-3.5 w-3.5" />
      </Button>
    </div>
  )
}

function LogTableSkeleton({ rows = 8 }: { rows?: number }) {
  const { t } = useTranslation("logs")
  const skeletonRows = rows > 8 ? SKELETON_ROWS_10 : SKELETON_ROWS_8
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>{t("columns.time")}</TableHead>
          <TableHead>{t("columns.model")}</TableHead>
          <TableHead>{t("columns.channel")}</TableHead>
          <TableHead className="text-right">{t("columns.input")}</TableHead>
          <TableHead className="text-right">{t("columns.output")}</TableHead>
          <TableHead className="text-right">{t("columns.ttft")}</TableHead>
          <TableHead className="text-right">{t("columns.latency")}</TableHead>
          <TableHead className="text-right">{t("columns.cost")}</TableHead>
          <TableHead>{t("columns.status")}</TableHead>
          <TableHead className="w-10" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {skeletonRows.map((_, i) => (
          <TableRow key={`log-sk-${i.toString()}`}>
            <TableCell>
              <Skeleton className="h-4 w-24" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-5 w-28 rounded-full" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-4 w-20" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-14" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-14" />
            </TableCell>
            <TableCell className="text-right">
              <Skeleton className="ml-auto h-4 w-12" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-5 w-10 rounded-full" />
            </TableCell>
            <TableCell>
              <Skeleton className="h-8 w-8 rounded-md" />
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

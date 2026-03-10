import { useQuery } from "@tanstack/react-query"
import { ChevronLeft, ChevronRight, Terminal } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listMCPLogs } from "@/lib/api"

const PAGE_SIZE = 50

export function McpLogTab() {
  const { t } = useTranslation("logs")
  const [page, setPage] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-logs", page],
    queryFn: () => listMCPLogs({ page, pageSize: PAGE_SIZE }),
  })

  const logs = data?.data?.logs ?? []
  const total = data?.data?.total ?? 0
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE))

  function formatTime(timestamp: number) {
    return new Date(timestamp * 1000).toLocaleString()
  }

  function formatDuration(ms: number) {
    if (ms < 1000) return `${ms}ms`
    return `${(ms / 1000).toFixed(2)}s`
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <p className="text-muted-foreground mb-4 text-sm">{t("mcp.description")}</p>

      <div className="overflow-x-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("mcp.columns.time")}</TableHead>
              <TableHead>{t("mcp.columns.client")}</TableHead>
              <TableHead>{t("mcp.columns.tool")}</TableHead>
              <TableHead>{t("mcp.columns.status")}</TableHead>
              <TableHead>{t("mcp.columns.duration")}</TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5}>
                  <div className="flex justify-center py-8">
                    <p className="text-muted-foreground">
                      {t("actions.loading", { ns: "common" })}
                    </p>
                  </div>
                </TableCell>
              </TableRow>
            ) : logs.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5}>
                  <div className="flex flex-col items-center justify-center gap-2 py-16">
                    <Terminal className="text-muted-foreground h-10 w-10" />
                    <p className="text-muted-foreground font-medium">{t("mcp.empty")}</p>
                    <p className="text-muted-foreground text-sm">{t("mcp.emptyHint")}</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="whitespace-nowrap">{formatTime(log.time)}</TableCell>
                  <TableCell>{log.clientName}</TableCell>
                  <TableCell>
                    <code className="text-xs">{log.toolName}</code>
                  </TableCell>
                  <TableCell>
                    <Badge variant={log.status === "ok" ? "default" : "destructive"}>
                      {log.status}
                    </Badge>
                  </TableCell>
                  <TableCell>{formatDuration(log.duration)}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-end gap-2 pt-3">
          <span className="text-muted-foreground text-sm tabular-nums">
            {page}/{totalPages}
          </span>
          <Button
            variant="outline"
            size="icon-xs"
            disabled={page <= 1}
            onClick={() => setPage((p) => p - 1)}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </Button>
          <Button
            variant="outline"
            size="icon-xs"
            disabled={page >= totalPages}
            onClick={() => setPage((p) => p + 1)}
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </Button>
        </div>
      )}
    </div>
  )
}

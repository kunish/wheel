import { useQuery } from "@tanstack/react-query"
import { Terminal } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listMCPLogs } from "@/lib/api-client"

export function McpLogTab() {
  const { t } = useTranslation("logs")
  const [page] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-logs", page],
    queryFn: () => listMCPLogs({ page, pageSize: 50 }),
  })

  const logs = data?.data?.logs ?? []

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
    </div>
  )
}

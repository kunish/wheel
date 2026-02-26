import { useQuery } from "@tanstack/react-query"
import { ClipboardList } from "lucide-react"
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
import { listAuditLogs } from "@/lib/api-client"

export function AuditLogTab() {
  const { t } = useTranslation("logs")
  const [page] = useState(1)

  const { data, isLoading } = useQuery({
    queryKey: ["audit-logs", page],
    queryFn: () => listAuditLogs({ page, pageSize: 50 }),
  })

  const logs = data?.data?.logs ?? []

  function formatTime(timestamp: number) {
    return new Date(timestamp * 1000).toLocaleString()
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <p className="text-muted-foreground mb-4 text-sm">{t("audit.description")}</p>

      <div className="overflow-x-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>{t("audit.columns.time")}</TableHead>
              <TableHead>{t("audit.columns.user")}</TableHead>
              <TableHead>{t("audit.columns.action")}</TableHead>
              <TableHead>{t("audit.columns.target")}</TableHead>
              <TableHead>{t("audit.columns.detail")}</TableHead>
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
                    <ClipboardList className="text-muted-foreground h-10 w-10" />
                    <p className="text-muted-foreground font-medium">{t("audit.empty")}</p>
                    <p className="text-muted-foreground text-sm">{t("audit.emptyHint")}</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              logs.map((log) => (
                <TableRow key={log.id}>
                  <TableCell className="whitespace-nowrap">{formatTime(log.time)}</TableCell>
                  <TableCell>{log.user}</TableCell>
                  <TableCell>
                    <Badge variant="outline">{log.action}</Badge>
                  </TableCell>
                  <TableCell>
                    <code className="text-xs">{log.target}</code>
                  </TableCell>
                  <TableCell className="max-w-xs truncate">{log.detail}</TableCell>
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>
    </div>
  )
}

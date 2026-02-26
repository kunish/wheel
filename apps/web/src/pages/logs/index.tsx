import type { LogEntry } from "./columns"
import { ArrowUp } from "lucide-react"
import { useCallback, useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useLogQuery } from "@/hooks/use-log-query"
import { AuditLogTab } from "./audit-log-tab"
import { LogDetailSheet } from "./log-detail-panel"
import { LogFilterBar } from "./log-filter-bar"
import { LogTable, PaginationControls } from "./log-table"
import { McpLogTab } from "./mcp-log-tab"

export default function LogsPage() {
  const { t } = useTranslation("logs")
  const q = useLogQuery()
  const [activeTab, setActiveTab] = useState("requests")

  const handleRowClick = useCallback(
    (log: LogEntry) => {
      if (log._streaming && log._streamId) {
        q.setDetailStreamId(log._streamId)
      } else {
        q.setDetailStreamId(null)
        q.setDetailId(log.id)
      }
    },
    [q],
  )

  const handleNavigate = useCallback(
    (log: LogEntry) => {
      if (log._streaming && log._streamId) {
        q.setDetailId(null)
        q.setDetailStreamId(log._streamId)
      } else {
        q.setDetailStreamId(null)
        q.setDetailId(log.id)
      }
    },
    [q],
  )

  const handleClearAll = useCallback(() => {
    q.navigate(q.pathname, { replace: true })
    q.setKeywordInput("")
  }, [q])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="bg-background shrink-0 pb-4">
        <div className="flex items-center justify-between">
          <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        </div>
      </div>

      <Tabs value={activeTab} onValueChange={setActiveTab} className="flex min-h-0 flex-1 flex-col">
        <TabsList variant="line" className="shrink-0">
          <TabsTrigger value="requests">{t("tabs.requests")}</TabsTrigger>
          <TabsTrigger value="audit">{t("tabs.audit")}</TabsTrigger>
          <TabsTrigger value="mcp">{t("tabs.mcp")}</TabsTrigger>
        </TabsList>

        <TabsContent value="requests" className="flex min-h-0 flex-1 flex-col pt-4">
          {/* Request logs header: count + pagination + filters */}
          <div className="bg-background shrink-0 space-y-4 pb-4">
            <div className="flex items-center justify-between">
              <div className="flex items-baseline gap-3">
                <span className="text-muted-foreground text-sm">
                  {t("totalCount", { count: q.total })}
                </span>
                {q.pendingCount > 0 && (
                  <Button
                    variant="outline"
                    size="xs"
                    className="animate-pulse gap-1"
                    onClick={q.handleShowNew}
                  >
                    <ArrowUp className="h-3 w-3" />
                    {t("newLogs", { count: q.pendingCount })}
                  </Button>
                )}
              </div>
              {q.totalPages > 0 && (
                <PaginationControls
                  page={q.page}
                  pageSize={q.pageSize}
                  totalPages={q.totalPages}
                  updateFilter={q.updateFilter}
                />
              )}
            </div>

            <LogFilterBar
              keyword={q.keyword}
              keywordInput={q.keywordInput}
              setKeywordInput={q.setKeywordInput}
              model={q.model}
              status={q.status}
              channelId={q.channelId}
              startTime={q.startTime}
              endTime={q.endTime}
              hasFilters={q.hasFilters}
              channels={q.channels}
              modelOptions={q.modelOptions}
              updateFilter={q.updateFilter}
              debouncedUpdateFilter={q.debouncedUpdateFilter}
              onClearAll={handleClearAll}
            />
          </div>

          <LogTable
            logs={q.logs}
            pageSize={q.pageSize}
            isLoading={q.isLoading}
            isFetching={q.isFetching}
            isError={q.isError}
            hasFilters={q.hasFilters}
            refetch={q.refetch}
            onViewDetail={q.setDetailId}
            onRowClick={handleRowClick}
            onClearFilters={handleClearAll}
          />

          <LogDetailSheet
            detailId={q.detailId}
            detailStreamId={q.detailStreamId}
            detailTab={q.detailTab}
            detail={q.detail}
            pendingStreams={q.pendingStreams}
            streamingOverlay={q.streamingOverlay}
            logs={q.logs}
            onClose={() => {
              q.setDetailId(null)
              q.setDetailStreamId(null)
            }}
            onNavigate={handleNavigate}
            onTabChange={q.setDetailTab}
          />
        </TabsContent>

        <TabsContent value="audit" className="flex min-h-0 flex-1 flex-col pt-4">
          <AuditLogTab />
        </TabsContent>

        <TabsContent value="mcp" className="flex min-h-0 flex-1 flex-col pt-4">
          <McpLogTab />
        </TabsContent>
      </Tabs>
    </div>
  )
}

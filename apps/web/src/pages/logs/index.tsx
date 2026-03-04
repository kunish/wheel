import { ArrowUp } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { LogStreamIndicator } from "@/components/log-stream-indicator"
import { Button } from "@/components/ui/button"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { AuditLogTab } from "./audit-log-tab"
import { LogDetailSheet } from "./log-detail-panel"
import { LogFilterBar } from "./log-filter-bar"
import { LogQueryProvider, useLogQueryContext } from "./log-query-context"
import { LogTable, PaginationControls } from "./log-table"
import { McpLogTab } from "./mcp-log-tab"

export default function LogsPage() {
  return (
    <LogQueryProvider>
      <LogsPageContent />
    </LogQueryProvider>
  )
}

function LogsPageContent() {
  const { t } = useTranslation("logs")
  const {
    total,
    pendingCount,
    pendingStreams,
    isPaused,
    connectionState,
    togglePause,
    handleShowNew,
    totalPages,
    page,
    pageSize,
    updateFilter,
  } = useLogQueryContext()
  const [activeTab, setActiveTab] = useState("requests")

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
                  {t("totalCount", { count: total })}
                </span>
                <LogStreamIndicator
                  isLive
                  isPaused={isPaused}
                  streamCount={pendingStreams.size}
                  connectionState={connectionState}
                  onTogglePause={togglePause}
                />
                {pendingCount > 0 && (
                  <Button
                    variant="outline"
                    size="xs"
                    className="animate-pulse gap-1"
                    onClick={handleShowNew}
                  >
                    <ArrowUp className="h-3 w-3" />
                    {t("newLogs", { count: pendingCount })}
                  </Button>
                )}
              </div>
              {totalPages > 0 && (
                <PaginationControls
                  page={page}
                  pageSize={pageSize}
                  totalPages={totalPages}
                  updateFilter={updateFilter}
                />
              )}
            </div>

            <LogFilterBar />
          </div>

          <LogTable />

          <LogDetailSheet />
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

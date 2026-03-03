import type { TimelineItem } from "@/hooks/use-playground-chat"
import { CheckCircle2, Clock3, Loader2, PlayCircle, Wrench, XCircle } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"

interface ToolCallTimelineProps {
  mode: "auto" | "manual"
  items: TimelineItem[]
  isLoading: boolean
  canContinueManual: boolean
  onExecuteTool: (toolCallId: string) => void
  onContinue: () => void
}

function statusBadge(
  status: TimelineItem["status"],
  labels: { done: string; running: string; error: string; pending: string },
) {
  if (status === "done") {
    return (
      <Badge variant="default" className="gap-1 text-[10px]">
        <CheckCircle2 className="h-3 w-3" />
        {labels.done}
      </Badge>
    )
  }
  if (status === "running") {
    return (
      <Badge variant="secondary" className="gap-1 text-[10px]">
        <Loader2 className="h-3 w-3 animate-spin" />
        {labels.running}
      </Badge>
    )
  }
  if (status === "error") {
    return (
      <Badge variant="destructive" className="gap-1 text-[10px]">
        <XCircle className="h-3 w-3" />
        {labels.error}
      </Badge>
    )
  }
  return (
    <Badge variant="outline" className="gap-1 text-[10px]">
      <Clock3 className="h-3 w-3" />
      {labels.pending}
    </Badge>
  )
}

export function ToolCallTimeline({
  mode,
  items,
  isLoading,
  canContinueManual,
  onExecuteTool,
  onContinue,
}: ToolCallTimelineProps) {
  const { t } = useTranslation("playground")
  const statusLabels = {
    done: t("timeline.done"),
    running: t("timeline.running"),
    error: t("timeline.error"),
    pending: t("timeline.pending"),
  }

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Wrench className="h-4 w-4" />
          {t("timeline.title")}
          <Badge variant="outline" className="ml-auto text-[10px]">
            {items.length}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        {items.length === 0 ? (
          <p className="text-muted-foreground text-xs">{t("timeline.empty")}</p>
        ) : (
          <div className="space-y-2">
            {items.map((item) => (
              <div key={item.id} className="rounded-md border p-2">
                <div className="flex items-center justify-between gap-2">
                  <div className="min-w-0">
                    <p className="truncate text-xs font-medium">{item.title}</p>
                    <p className="text-muted-foreground truncate font-mono text-[11px]">
                      {item.alias}
                    </p>
                  </div>
                  {statusBadge(item.status, statusLabels)}
                </div>
                <pre className="bg-muted mt-2 overflow-x-auto rounded p-2 text-[11px] whitespace-pre-wrap">
                  {JSON.stringify(item.argumentsObj, null, 2)}
                </pre>
                {item.resultText && (
                  <pre className="bg-muted mt-2 overflow-x-auto rounded p-2 text-[11px] whitespace-pre-wrap">
                    {item.resultText}
                  </pre>
                )}
                {item.error && <p className="text-destructive mt-2 text-xs">{item.error}</p>}
                {mode === "manual" && item.status === "pending" && (
                  <Button
                    type="button"
                    size="sm"
                    variant="outline"
                    className="mt-2 h-7"
                    onClick={() => onExecuteTool(item.callId)}
                  >
                    {t("timeline.execute")}
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}

        {mode === "manual" && items.length > 0 && (
          <Button
            type="button"
            className="w-full"
            onClick={onContinue}
            disabled={!canContinueManual || isLoading}
          >
            <PlayCircle className="mr-2 h-4 w-4" />
            {t("timeline.continue")}
          </Button>
        )}
      </CardContent>
    </Card>
  )
}

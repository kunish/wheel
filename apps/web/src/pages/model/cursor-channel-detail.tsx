import { useMutation, useQueryClient } from "@tanstack/react-query"
import { Loader2, RefreshCw } from "lucide-react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { refreshCursorChannelModels } from "@/lib/api/channels"
import { channelsQueryKey } from "./codex-query-keys"

export function CursorChannelDetail({ channelId }: { channelId: number }) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()

  const refreshMut = useMutation({
    mutationFn: () => refreshCursorChannelModels(channelId),
    onSuccess: (res) => {
      void queryClient.invalidateQueries({ queryKey: channelsQueryKey })
      const data = res.data
      if (data.unchanged) {
        toast.info(t("cursor.refreshUnchanged"))
        return
      }
      toast.success(t("cursor.refreshSuccess", { count: data.count ?? data.models?.length ?? 0 }))
    },
    onError: (err: Error) => {
      toast.error(err.message || t("cursor.refreshFailed"))
    },
  })

  return (
    <div className="mt-2 space-y-2">
      <h5 className="text-muted-foreground shrink-0 text-xs font-medium tracking-wide whitespace-nowrap uppercase">
        {t("cursor.managementTitle")}
      </h5>
      <p className="text-muted-foreground text-xs">{t("cursor.refreshHint")}</p>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="h-7 text-xs"
        onClick={() => refreshMut.mutate()}
        disabled={refreshMut.isPending}
      >
        {refreshMut.isPending ? (
          <Loader2 className="mr-1 h-3 w-3 animate-spin" />
        ) : (
          <RefreshCw className="mr-1 h-3 w-3" />
        )}
        {t("codex.refresh")}
      </Button>
    </div>
  )
}

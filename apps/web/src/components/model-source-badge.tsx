import { useTranslation } from "react-i18next"
import { useModelMeta } from "@/hooks/use-model-meta"

/**
 * Displays a small indicator showing the source of a model:
 * - Green dot: fetched from the channel's API
 * - Blue dot: recognized by models.dev (but not API-fetched)
 * - Gray text "manual": unknown / manually typed
 */
export function ModelSourceBadge({
  modelId,
  isApiFetched,
}: {
  modelId: string
  isApiFetched: boolean
}) {
  const { t } = useTranslation("model")
  const meta = useModelMeta(modelId)

  if (isApiFetched) {
    return (
      <span
        className="h-2 w-2 shrink-0 rounded-full bg-green-500"
        title={t("channelModelPicker.apiVerified")}
      />
    )
  }

  if (meta) {
    return (
      <span
        className="h-2 w-2 shrink-0 rounded-full bg-blue-500"
        title={t("channelModelPicker.modelsdev")}
      />
    )
  }

  return (
    <span className="text-muted-foreground/60 shrink-0 text-[10px]">
      {t("channelModelPicker.manual")}
    </span>
  )
}

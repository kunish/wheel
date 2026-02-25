import { motion } from "motion/react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ModelSourceBadge } from "@/components/model-source-badge"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import { ModelPickerBase } from "./model-picker-base"

export interface ChannelModelEntry {
  channelId: number
  channelName: string
  models: string[]
  fetchedModels: string[]
}

interface ChannelModelPickerDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelModels: ChannelModelEntry[]
  onSelect: (channelId: number, modelId: string) => void
}

export default function ChannelModelPickerDialog({
  open,
  onOpenChange,
  channelModels,
  onSelect,
}: ChannelModelPickerDialogProps) {
  const { t } = useTranslation("model")
  const { data } = useModelMetadataQuery()
  const [search, setSearch] = useState("")
  const [channelFilter, setChannelFilter] = useState<number | null>(null)

  const grouped = useMemo(() => {
    const rawMap = data?.data
    const q = search.toLowerCase()
    const result: Record<
      string,
      {
        modelId: string
        channelId: number
        channelName: string
        logoUrl: string
        isApiFetched: boolean
      }[]
    > = {}

    for (const ch of channelModels) {
      if (channelFilter !== null && ch.channelId !== channelFilter) continue
      const fetchedSet = new Set(ch.fetchedModels ?? [])

      for (const modelId of ch.models) {
        const meta = rawMap ? fuzzyLookup(rawMap, modelId) : null
        const providerName = meta?.providerName || "Other"

        if (q) {
          const idMatch = modelId.toLowerCase().includes(q)
          const nameMatch = meta?.name.toLowerCase().includes(q)
          const providerMatch = providerName.toLowerCase().includes(q)
          const channelMatch = ch.channelName.toLowerCase().includes(q)
          if (!idMatch && !nameMatch && !providerMatch && !channelMatch) continue
        }

        if (!result[providerName]) result[providerName] = []
        result[providerName].push({
          modelId,
          channelId: ch.channelId,
          channelName: ch.channelName,
          logoUrl: meta?.logoUrl || "",
          isApiFetched: fetchedSet.has(modelId),
        })
      }
    }

    for (const items of Object.values(result)) {
      items.sort((a, b) => a.modelId.localeCompare(b.modelId))
    }
    return result
  }, [data, channelModels, search, channelFilter])

  const providerKeys = useMemo(
    () =>
      Object.keys(grouped)
        .sort()
        .sort((a, b) => (a === "Other" ? 1 : b === "Other" ? -1 : 0)),
    [grouped],
  )

  const totalCount = useMemo(
    () => providerKeys.reduce((s, k) => s + grouped[k].length, 0),
    [grouped, providerKeys],
  )

  return (
    <ModelPickerBase
      open={open}
      onOpenChange={onOpenChange}
      title={t("channelModelPicker.title")}
      searchPlaceholder={t("channelModelPicker.searchPlaceholder")}
      emptyText={
        channelModels.length === 0
          ? t("channelModelPicker.noChannelsConfigured")
          : t("channelModelPicker.noModelsFound")
      }
      search={search}
      onSearchChange={setSearch}
      providerKeys={providerKeys}
      getProviderCount={(p) => grouped[p].length}
      totalCount={totalCount}
      extraControls={
        <Select
          value={channelFilter !== null ? String(channelFilter) : "all"}
          onValueChange={(v) => setChannelFilter(v === "all" ? null : Number(v))}
        >
          <SelectTrigger className="w-40">
            <SelectValue placeholder={t("channelModelPicker.allChannels")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("channelModelPicker.allChannels")}</SelectItem>
            {channelModels.map((ch) => (
              <SelectItem key={ch.channelId} value={String(ch.channelId)}>
                {ch.channelName}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      }
      renderProviderItems={(provider) =>
        grouped[provider].map((m) => (
          <motion.button
            key={`${m.channelId}-${m.modelId}`}
            initial={{ opacity: 0, height: 0 }}
            animate={{ opacity: 1, height: "auto" }}
            exit={{ opacity: 0, height: 0 }}
            transition={{ duration: 0.2 }}
            type="button"
            className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 overflow-hidden rounded-md px-2 py-1.5 text-left text-sm transition-colors"
            onClick={() => onSelect(m.channelId, m.modelId)}
          >
            {m.logoUrl && (
              <img
                src={m.logoUrl}
                alt=""
                width={16}
                height={16}
                className="shrink-0 dark:invert"
                onError={(e) => {
                  ;(e.target as HTMLImageElement).style.display = "none"
                }}
              />
            )}
            <span className="flex-1 truncate">{m.modelId}</span>
            <ModelSourceBadge modelId={m.modelId} isApiFetched={m.isApiFetched} />
            <span className="text-muted-foreground shrink-0 text-xs">{m.channelName}</span>
          </motion.button>
        ))
      }
    />
  )
}

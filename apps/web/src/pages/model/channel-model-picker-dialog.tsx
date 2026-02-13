import { Search } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"

export interface ChannelModelEntry {
  channelId: number
  channelName: string
  models: string[]
  fetchedModels: string[]
}

export default function ChannelModelPickerDialog({
  open,
  onOpenChange,
  channelModels,
  onSelect,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  channelModels: ChannelModelEntry[]
  onSelect: (channelId: number, modelId: string) => void
}) {
  const { t } = useTranslation("model")
  const { data } = useModelMetadataQuery()
  const [search, setSearch] = useState("")
  const [channelFilter, setChannelFilter] = useState<number | null>(null)

  // Flatten channel→model pairs, group by provider
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
      // Quick filter by channel
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
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg overflow-hidden">
        <DialogHeader>
          <DialogTitle>{t("channelModelPicker.title")}</DialogTitle>
        </DialogHeader>

        <div className="flex items-center gap-2">
          <div className="relative flex-1">
            <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
            <Input
              placeholder={t("channelModelPicker.searchPlaceholder")}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="pl-9"
            />
          </div>
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
        </div>

        <ScrollArea className="h-[50vh]">
          {totalCount === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">
              {channelModels.length === 0
                ? t("channelModelPicker.noChannelsConfigured")
                : t("channelModelPicker.noModelsFound")}
            </p>
          ) : (
            <div className="flex flex-col gap-3 pr-3">
              {providerKeys.map((provider) => (
                <div key={provider}>
                  <p className="text-muted-foreground mb-1.5 px-1 text-xs font-semibold">
                    {provider}
                    <span className="text-muted-foreground/60 ml-1">
                      ({grouped[provider].length})
                    </span>
                  </p>
                  <div className="flex flex-col gap-0.5">
                    {grouped[provider].map((m) => (
                      <button
                        key={`${m.channelId}-${m.modelId}`}
                        type="button"
                        className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
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
                        <span className="text-muted-foreground shrink-0 text-xs">
                          {m.channelName}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

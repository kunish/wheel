import type { ReactNode } from "react"
import type { ModelMeta } from "@/lib/api"
import { useMemo, useState } from "react"
import { fuzzyLookup, useModelMetadataQuery } from "@/hooks/use-model-meta"
import { groupModelsByProvider } from "@/lib/group-models"
import { cn } from "@/lib/utils"

const GROUP_THRESHOLD = 4

interface GroupedModelListProps {
  models: string[]
  renderModel: (modelId: string) => ReactNode
  className?: string
}

export function GroupedModelList({ models, renderModel, className }: GroupedModelListProps) {
  const { data } = useModelMetadataQuery()
  const rawMap = data?.data

  // Build a resolved map using fuzzy matching so grouping works for variants
  const resolvedMap = useMemo(() => {
    if (!rawMap) return undefined
    const map: Record<string, ModelMeta> = {}
    for (const id of models) {
      const meta = fuzzyLookup(rawMap, id)
      if (meta) map[id] = meta
    }
    return map
  }, [rawMap, models])

  const groups = useMemo(() => groupModelsByProvider(models, resolvedMap), [models, resolvedMap])

  // Flat list for few models
  if (models.length < GROUP_THRESHOLD) {
    return (
      <div
        className={cn("grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-1.5", className)}
      >
        {models.map((m) => (
          <div key={m}>{renderModel(m)}</div>
        ))}
      </div>
    )
  }

  return (
    <div className={cn("flex flex-col gap-2", className)}>
      {groups.map((group) => (
        <ProviderSection key={group.provider} group={group} renderModel={renderModel} />
      ))}
    </div>
  )
}

function ProviderSection({
  group,
  renderModel,
}: {
  group: ReturnType<typeof groupModelsByProvider>[number]
  renderModel: (modelId: string) => ReactNode
}) {
  const [logoError, setLogoError] = useState(false)
  const showLogo = group.logoUrl && !logoError

  return (
    <div>
      <div className="mb-1 flex items-center gap-1.5">
        {showLogo && (
          <img
            src={group.logoUrl!}
            alt={group.providerName}
            width={16}
            height={16}
            className="shrink-0 dark:invert"
            onError={() => setLogoError(true)}
          />
        )}
        <span className="text-muted-foreground text-xs">
          {group.providerName} ({group.models.length})
        </span>
      </div>
      <div className="grid grid-cols-[repeat(auto-fill,minmax(180px,1fr))] gap-1.5">
        {group.models.map((m) => (
          <div key={m}>{renderModel(m)}</div>
        ))}
      </div>
    </div>
  )
}

"use client"

import { useState } from "react"
import { useModelMeta } from "@/hooks/use-model-meta"

export function ModelBadge({ modelId }: { modelId: string }) {
  const meta = useModelMeta(modelId)
  const [logoError, setLogoError] = useState(false)

  const displayName = meta?.name ?? modelId
  const showLogo = meta && !logoError

  return (
    <span className="inline-flex max-w-full items-center gap-1.5">
      {showLogo && (
        <img
          src={meta.logoUrl}
          alt={meta.providerName}
          width={16}
          height={16}
          className="shrink-0 dark:invert"
          onError={() => setLogoError(true)}
        />
      )}
      <span className="truncate">{displayName}</span>
    </span>
  )
}

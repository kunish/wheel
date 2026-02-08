"use client"

import { X } from "lucide-react"
import { useState } from "react"
import { useModelMeta } from "@/hooks/use-model-meta"
import { cn } from "@/lib/utils"

interface ModelCardProps extends React.HTMLAttributes<HTMLDivElement> {
  modelId: string
  disabled?: boolean
  onRemove?: () => void
}

export const ModelCard = function ModelCard({
  ref,
  modelId,
  disabled,
  onRemove,
  className,
  children,
  ...props
}: ModelCardProps & { ref?: React.RefObject<HTMLDivElement | null> }) {
  const meta = useModelMeta(modelId)
  const [logoError, setLogoError] = useState(false)

  const displayName = meta?.name ?? modelId
  const showLogo = meta && !logoError

  return (
    <div
      ref={ref}
      className={cn(
        "bg-card hover:bg-accent hover:text-accent-foreground inline-flex items-center gap-2 rounded-lg border px-2.5 py-1.5 text-sm transition-colors",
        disabled && "pointer-events-none opacity-40",
        className,
      )}
      {...props}
    >
      {showLogo && (
        <img
          src={meta.logoUrl}
          alt={meta.providerName}
          width={20}
          height={20}
          className="shrink-0 dark:invert"
          onError={() => setLogoError(true)}
        />
      )}
      <div className="flex min-w-0 flex-col items-start">
        <span className="truncate text-xs leading-tight font-medium">{displayName}</span>
        {meta && displayName !== modelId && (
          <span className="text-muted-foreground truncate font-mono text-[10px] leading-tight">
            {modelId}
          </span>
        )}
        {meta?.providerName && (
          <span className="text-muted-foreground truncate text-[10px] leading-tight">
            {meta.providerName}
          </span>
        )}
      </div>
      {children}
      {onRemove && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onRemove()
          }}
          className="hover:bg-muted-foreground/20 ml-auto shrink-0 rounded-full p-0.5"
        >
          <X className="h-3 w-3" />
        </button>
      )}
    </div>
  )
}

import { X } from "lucide-react"
import { useState } from "react"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { useModelMeta } from "@/hooks/use-model-meta"
import { cn } from "@/lib/utils"

export interface ModelPriceInfo {
  inputPrice: number
  outputPrice: number
}

interface ModelCardProps extends React.HTMLAttributes<HTMLDivElement> {
  modelId: string
  disabled?: boolean
  onRemove?: () => void
  price?: ModelPriceInfo
  onPriceClick?: () => void
}

export const ModelCard = function ModelCard({
  ref,
  modelId,
  disabled,
  onRemove,
  price,
  onPriceClick,
  className,
  children,
  ...props
}: ModelCardProps & { ref?: React.Ref<HTMLDivElement | null> }) {
  const meta = useModelMeta(modelId)
  const [logoError, setLogoError] = useState(false)

  const displayName = meta?.name ?? modelId
  const showLogo = meta && !logoError

  const card = (
    <div
      ref={ref}
      className={cn(
        "bg-card hover:bg-accent hover:text-accent-foreground flex min-w-0 items-center gap-2 overflow-hidden rounded-lg border px-2.5 py-1.5 text-sm transition-colors",
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
      <div className="flex min-w-0 flex-1 flex-col items-start">
        <span className="w-full truncate text-xs leading-tight font-medium">{displayName}</span>
        {meta && displayName !== modelId && (
          <span className="text-muted-foreground w-full truncate font-mono text-[10px] leading-tight">
            {modelId}
          </span>
        )}
        {meta?.providerName && (
          <span className="text-muted-foreground w-full truncate text-[10px] leading-tight">
            {meta.providerName}
          </span>
        )}
      </div>
      {price && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation()
            onPriceClick?.()
          }}
          className="text-muted-foreground hover:text-foreground shrink-0 font-mono text-[10px] leading-tight transition-colors"
        >
          ↓{price.inputPrice.toFixed(2)} ↑{price.outputPrice.toFixed(2)}
        </button>
      )}
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

  return (
    <TooltipProvider>
      <Tooltip>
        <TooltipTrigger asChild>{card}</TooltipTrigger>
        <TooltipContent>
          <div className="flex flex-col gap-0.5">
            <span className="font-medium">{displayName}</span>
            {displayName !== modelId && (
              <span className="font-mono font-normal opacity-80">{modelId}</span>
            )}
            {price && (
              <span className="font-mono text-xs font-normal opacity-80">
                Input: ${price.inputPrice.toFixed(6)}/M · Output: ${price.outputPrice.toFixed(6)}/M
              </span>
            )}
          </div>
        </TooltipContent>
      </Tooltip>
    </TooltipProvider>
  )
}

import type { ReactNode } from "react"
import { Filter, X } from "lucide-react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { cn } from "@/lib/utils"

export interface FilterChip {
  key: string
  label: string
  onRemove: () => void
}

interface FilterPopoverProps {
  /** Number of active filters */
  activeCount?: number
  /** Label for the trigger button */
  label?: string
  /** Filter chips to display below the trigger */
  chips?: FilterChip[]
  /** Called when "Clear all" is clicked */
  onClearAll?: () => void
  /** Popover content (the filter controls) */
  children: ReactNode
  /** Additional class for the trigger */
  className?: string
  /** Alignment of the popover */
  align?: "start" | "center" | "end"
}

export function FilterPopover({
  activeCount = 0,
  label = "Filters",
  chips,
  onClearAll,
  children,
  className,
  align = "start",
}: FilterPopoverProps) {
  return (
    <div className="flex flex-col gap-2">
      <Popover>
        <PopoverTrigger asChild>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className={cn("h-8 gap-1.5 text-xs", activeCount > 0 && "border-primary", className)}
          >
            <Filter className="h-3.5 w-3.5" />
            {label}
            {activeCount > 0 && (
              <Badge variant="secondary" className="ml-0.5 h-4 min-w-4 px-1 text-[10px]">
                {activeCount}
              </Badge>
            )}
          </Button>
        </PopoverTrigger>
        <PopoverContent align={align} className="w-72 space-y-3 p-4">
          {children}
        </PopoverContent>
      </Popover>

      {chips && chips.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {chips.map((chip) => (
            <Badge key={chip.key} variant="secondary" className="gap-1 pr-1">
              {chip.label}
              <button type="button" onClick={chip.onRemove}>
                <X className="h-3 w-3" />
              </button>
            </Badge>
          ))}
          {onClearAll && (
            <Button
              type="button"
              variant="ghost"
              size="sm"
              className="h-6 px-2 text-xs"
              onClick={onClearAll}
            >
              Clear all
            </Button>
          )}
        </div>
      )}
    </div>
  )
}

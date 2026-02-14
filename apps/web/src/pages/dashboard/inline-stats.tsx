import type { MessageSquare } from "lucide-react"
import type { formatCount } from "@/lib/format"
import { AnimatedNumber } from "@/components/animated-number"

interface InlineStatsItem {
  label: string
  raw: number
  format: typeof formatCount
  icon: typeof MessageSquare
}

export function InlineStats({ items }: { items: InlineStatsItem[] }) {
  return (
    <div className="flex flex-wrap items-center justify-center gap-x-4 gap-y-1.5 py-2">
      {items.map((item) => {
        const formatted = item.format(item.raw)
        return (
          <div key={item.label} className="flex items-center gap-1.5">
            <item.icon className="text-muted-foreground h-3 w-3 shrink-0" />
            <span className="text-muted-foreground text-[11px]">{item.label}</span>
            <span className="text-xs font-bold tabular-nums">
              <AnimatedNumber value={item.raw} formatter={(n) => item.format(n).value} />
              {formatted.unit && (
                <span className="text-muted-foreground ml-0.5 text-[10px] font-medium">
                  {formatted.unit}
                </span>
              )}
            </span>
          </div>
        )
      })}
    </div>
  )
}

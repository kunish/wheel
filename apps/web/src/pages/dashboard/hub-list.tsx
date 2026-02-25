import { useMemo, useState } from "react"
import { Link } from "react-router"

export interface HubListSortOption<K extends string> {
  key: K
  label: string
}

export interface HubListItem {
  id: string | number
  label: string
  link: string
  stats: React.ReactNode
}

interface HubListProps<K extends string> {
  sortOptions: HubListSortOption<K>[]
  defaultSort: K
  noDataMessage: string
  items: HubListItem[] | undefined
  getSortValue: (item: HubListItem, sortKey: K) => number
}

export function HubList<K extends string>({
  sortOptions,
  defaultSort,
  noDataMessage,
  items,
  getSortValue,
}: HubListProps<K>) {
  const [sortBy, setSortBy] = useState<K>(defaultSort)

  const sorted = useMemo(() => {
    if (!items) return []
    return [...items].sort((a, b) => getSortValue(b, sortBy) - getSortValue(a, sortBy))
  }, [items, sortBy, getSortValue])

  const maxVal = useMemo(() => {
    if (sorted.length === 0) return 1
    return getSortValue(sorted[0], sortBy) || 1
  }, [sorted, sortBy, getSortValue])

  if (!items || items.length === 0) {
    return <p className="text-muted-foreground text-center text-[10px]">{noDataMessage}</p>
  }

  return (
    <div className="flex flex-col gap-1">
      <div className="flex gap-0.5">
        {sortOptions.map((opt) => (
          <button
            key={opt.key}
            onClick={() => setSortBy(opt.key)}
            className={`rounded px-1 py-px text-[10px] font-bold transition-all ${
              sortBy === opt.key
                ? "bg-foreground/10 text-foreground"
                : "text-muted-foreground/50 hover:text-muted-foreground"
            }`}
          >
            {opt.label}
          </button>
        ))}
      </div>
      <div className="max-h-[90px] space-y-px overflow-y-auto">
        {sorted.map((item) => (
          <Link
            key={item.id}
            to={item.link}
            className="hover:bg-muted/40 relative block rounded px-1 py-0.5 transition-colors"
          >
            <div
              className="absolute inset-y-0 left-0 rounded opacity-[0.07]"
              style={{
                width: `${(getSortValue(item, sortBy) / maxVal) * 100}%`,
                backgroundColor: "var(--primary)",
              }}
            />
            <div className="relative flex items-center justify-between gap-1">
              <span className="min-w-0 truncate text-[10px] font-medium">{item.label}</span>
              <span className="text-muted-foreground flex shrink-0 gap-1.5 text-[10px] tabular-nums">
                {item.stats}
              </span>
            </div>
          </Link>
        ))}
      </div>
    </div>
  )
}

import { X } from "lucide-react"
import { useCallback, useMemo, useRef, useState } from "react"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { cn } from "@/lib/utils"

export interface MultiSelectOption {
  value: string
  label: string
}

interface MultiSelectProps {
  options: MultiSelectOption[]
  value: string[]
  onChange: (value: string[]) => void
  placeholder?: string
  searchPlaceholder?: string
  emptyText?: string
  className?: string
  maxDisplay?: number
}

export function MultiSelect({
  options,
  value,
  onChange,
  placeholder = "Select...",
  searchPlaceholder = "Search...",
  emptyText = "No results.",
  className,
  maxDisplay = 3,
}: MultiSelectProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const inputRef = useRef<HTMLInputElement>(null)

  const filtered = useMemo(() => {
    if (!search) return options
    const q = search.toLowerCase()
    return options.filter(
      (o) => o.label.toLowerCase().includes(q) || o.value.toLowerCase().includes(q),
    )
  }, [options, search])

  const toggle = useCallback(
    (v: string) => {
      onChange(value.includes(v) ? value.filter((x) => x !== v) : [...value, v])
    },
    [value, onChange],
  )

  const remove = useCallback(
    (v: string) => {
      onChange(value.filter((x) => x !== v))
    },
    [value, onChange],
  )

  const selectedLabels = useMemo(() => {
    const map = new Map(options.map((o) => [o.value, o.label]))
    return value.map((v) => ({ value: v, label: map.get(v) ?? v }))
  }, [value, options])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          role="combobox"
          aria-expanded={open}
          className={cn(
            "h-auto min-h-9 w-full justify-start gap-1 px-3 py-1.5 font-normal",
            className,
          )}
        >
          {selectedLabels.length === 0 ? (
            <span className="text-muted-foreground">{placeholder}</span>
          ) : (
            <div className="flex flex-wrap gap-1">
              {selectedLabels.slice(0, maxDisplay).map((item) => (
                <Badge key={item.value} variant="secondary" className="gap-0.5 pr-0.5 text-xs">
                  {item.label}
                  <button
                    type="button"
                    className="hover:bg-muted rounded-sm p-0.5"
                    onClick={(e) => {
                      e.stopPropagation()
                      remove(item.value)
                    }}
                  >
                    <X className="h-3 w-3" />
                  </button>
                </Badge>
              ))}
              {selectedLabels.length > maxDisplay && (
                <Badge variant="secondary" className="text-xs">
                  +{selectedLabels.length - maxDisplay}
                </Badge>
              )}
            </div>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent className="w-[var(--radix-popover-trigger-width)] p-0" align="start">
        <div className="border-b p-2">
          <input
            ref={inputRef}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder={searchPlaceholder}
            className="placeholder:text-muted-foreground w-full bg-transparent text-sm outline-none"
          />
        </div>
        <div className="max-h-60 overflow-y-auto p-1">
          {filtered.length === 0 ? (
            <p className="text-muted-foreground py-4 text-center text-sm">{emptyText}</p>
          ) : (
            filtered.map((option) => {
              const selected = value.includes(option.value)
              return (
                <button
                  type="button"
                  key={option.value}
                  onClick={() => toggle(option.value)}
                  className={cn(
                    "flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm transition-colors",
                    "hover:bg-accent hover:text-accent-foreground",
                    selected && "bg-accent/50",
                  )}
                >
                  <div
                    className={cn(
                      "border-primary flex h-4 w-4 shrink-0 items-center justify-center rounded-sm border",
                      selected && "bg-primary text-primary-foreground",
                    )}
                  >
                    {selected && (
                      <svg
                        width="10"
                        height="10"
                        viewBox="0 0 10 10"
                        fill="none"
                        aria-hidden="true"
                      >
                        <path
                          d="M2 5L4 7L8 3"
                          stroke="currentColor"
                          strokeWidth="1.5"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      </svg>
                    )}
                  </div>
                  {option.label}
                </button>
              )
            })
          )}
        </div>
        {value.length > 0 && (
          <div className="border-t p-1">
            <button
              type="button"
              onClick={() => onChange([])}
              className="text-muted-foreground hover:text-foreground w-full rounded-sm px-2 py-1.5 text-center text-xs transition-colors"
            >
              Clear all
            </button>
          </div>
        )}
      </PopoverContent>
    </Popover>
  )
}

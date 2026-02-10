"use client"

import { Check, ChevronDown, Search } from "lucide-react"
import { useMemo, useState } from "react"
import { ModelBadge } from "@/components/model-badge"
import { Input } from "@/components/ui/input"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { cn } from "@/lib/utils"

interface ModelComboboxProps {
  models: string[]
  value: string
  onChange: (value: string) => void
  /** Text shown when no model is selected. Defaults to "All Models". */
  placeholder?: string
  /** If true, shows an "All" option that clears the selection. Defaults to true. */
  allowAll?: boolean
  className?: string
}

export function ModelCombobox({
  models,
  value,
  onChange,
  placeholder = "All Models",
  allowAll = true,
  className,
}: ModelComboboxProps) {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")

  const filtered = useMemo(() => {
    if (!search) return models
    const lower = search.toLowerCase()
    return models.filter((m) => m.toLowerCase().includes(lower))
  }, [models, search])

  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <button
          className={cn(
            "border-border data-[placeholder]:text-muted-foreground bg-background focus-visible:border-ring focus-visible:ring-ring/50 flex h-10 w-44 items-center justify-between gap-2 rounded-md border-2 px-3 py-2 text-sm font-bold whitespace-nowrap shadow-[2px_2px_0_var(--nb-shadow)] transition-all outline-none focus-visible:shadow-[4px_4px_0_var(--nb-shadow)] focus-visible:ring-[3px]",
            className,
          )}
        >
          {value ? (
            <span className="truncate">
              <ModelBadge modelId={value} />
            </span>
          ) : (
            <span className="text-muted-foreground">{placeholder}</span>
          )}
          <ChevronDown className="ml-1 h-3.5 w-3.5 shrink-0 opacity-50" />
        </button>
      </PopoverTrigger>
      <PopoverContent className="w-56 p-0" align="start">
        <div className="border-b p-2">
          <div className="relative">
            <Search className="text-muted-foreground absolute top-1/2 left-2.5 h-3.5 w-3.5 -translate-y-1/2" />
            <Input
              placeholder="Search models..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              className="h-8 pl-8 text-xs"
              autoFocus
            />
          </div>
        </div>
        <div className="max-h-60 overflow-y-auto p-1">
          {allowAll && (
            <button
              className={`hover:bg-accent/30 flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm ${
                !value ? "font-medium" : ""
              }`}
              onClick={() => {
                onChange("")
                setOpen(false)
                setSearch("")
              }}
            >
              <Check className={`h-3.5 w-3.5 ${!value ? "opacity-100" : "opacity-0"}`} />
              <span className="text-muted-foreground">{placeholder}</span>
            </button>
          )}
          {filtered.map((m) => (
            <button
              key={m}
              className={`hover:bg-accent/30 flex w-full items-center gap-2 rounded-sm px-2 py-1.5 text-sm ${
                value === m ? "font-medium" : ""
              }`}
              onClick={() => {
                onChange(m)
                setOpen(false)
                setSearch("")
              }}
            >
              <Check
                className={`h-3.5 w-3.5 shrink-0 ${value === m ? "opacity-100" : "opacity-0"}`}
              />
              <span className="truncate">
                <ModelBadge modelId={m} />
              </span>
            </button>
          ))}
          {filtered.length === 0 && (
            <p className="text-muted-foreground py-4 text-center text-xs">No models found</p>
          )}
        </div>
      </PopoverContent>
    </Popover>
  )
}

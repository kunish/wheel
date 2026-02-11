import { Loader2, Search } from "lucide-react"
import { useMemo, useState } from "react"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"
import { useModelMetadataQuery } from "@/hooks/use-model-meta"

export default function ModelPickerDialog({
  open,
  onOpenChange,
  onSelect,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  onSelect: (modelId: string) => void
}) {
  const { data, isLoading } = useModelMetadataQuery()
  const [search, setSearch] = useState("")

  const grouped = useMemo(() => {
    const allModels = data?.data
    if (!allModels) return {}
    const q = search.toLowerCase()
    const result: Record<string, { id: string; name: string; logoUrl: string }[]> = {}
    for (const [id, meta] of Object.entries(allModels)) {
      if (
        q &&
        !id.toLowerCase().includes(q) &&
        !meta.name.toLowerCase().includes(q) &&
        !meta.providerName.toLowerCase().includes(q)
      )
        continue
      const provider = meta.providerName || "Other"
      if (!result[provider]) result[provider] = []
      result[provider].push({ id, name: meta.name, logoUrl: meta.logoUrl })
    }
    // sort providers alphabetically, models alphabetically within
    for (const models of Object.values(result)) {
      models.sort((a, b) => a.id.localeCompare(b.id))
    }
    return result
  }, [data, search])

  const providerKeys = useMemo(() => Object.keys(grouped).sort(), [grouped])

  const totalCount = useMemo(
    () => providerKeys.reduce((s, k) => s + grouped[k].length, 0),
    [grouped, providerKeys],
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg overflow-hidden">
        <DialogHeader>
          <DialogTitle>Select Model from models.dev</DialogTitle>
        </DialogHeader>

        <div className="relative">
          <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
          <Input
            placeholder="Search models..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>

        <ScrollArea className="h-[50vh]">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
            </div>
          ) : totalCount === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">No models found</p>
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
                        key={m.id}
                        type="button"
                        className="hover:bg-accent hover:text-accent-foreground flex w-full items-center gap-2 rounded-md px-2 py-1.5 text-left text-sm transition-colors"
                        onClick={() => onSelect(m.id)}
                      >
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
                        <span className="truncate">{m.id}</span>
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

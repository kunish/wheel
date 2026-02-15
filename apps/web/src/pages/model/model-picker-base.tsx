import { Loader2, Search } from "lucide-react"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { ScrollArea } from "@/components/ui/scroll-area"

export interface ModelPickerBaseProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  searchPlaceholder: string
  emptyText: string
  isLoading?: boolean
  search: string
  onSearchChange: (value: string) => void
  providerKeys: string[]
  getProviderCount: (provider: string) => number
  renderProviderItems: (provider: string) => React.ReactNode
  totalCount: number
  extraControls?: React.ReactNode
}

export function ModelPickerBase({
  open,
  onOpenChange,
  title,
  searchPlaceholder,
  emptyText,
  isLoading,
  search,
  onSearchChange,
  providerKeys,
  getProviderCount,
  renderProviderItems,
  totalCount,
  extraControls,
}: ModelPickerBaseProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg overflow-hidden">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
        </DialogHeader>

        {extraControls ? (
          <div className="flex items-center gap-2">
            <div className="relative flex-1">
              <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
              <Input
                placeholder={searchPlaceholder}
                value={search}
                onChange={(e) => onSearchChange(e.target.value)}
                className="pl-9"
              />
            </div>
            {extraControls}
          </div>
        ) : (
          <div className="relative">
            <Search className="text-muted-foreground absolute top-2.5 left-2.5 h-4 w-4" />
            <Input
              placeholder={searchPlaceholder}
              value={search}
              onChange={(e) => onSearchChange(e.target.value)}
              className="pl-9"
            />
          </div>
        )}

        <ScrollArea className="h-[50vh]">
          {isLoading ? (
            <div className="flex items-center justify-center py-8">
              <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
            </div>
          ) : totalCount === 0 ? (
            <p className="text-muted-foreground py-8 text-center text-sm">{emptyText}</p>
          ) : (
            <div className="flex flex-col gap-3 pr-3">
              {providerKeys.map((provider) => (
                <div key={provider}>
                  <p className="text-muted-foreground mb-1.5 px-1 text-xs font-semibold">
                    {provider}
                    <span className="text-muted-foreground/60 ml-1">
                      ({getProviderCount(provider)})
                    </span>
                  </p>
                  <div className="flex flex-col gap-0.5">{renderProviderItems(provider)}</div>
                </div>
              ))}
            </div>
          )}
        </ScrollArea>
      </DialogContent>
    </Dialog>
  )
}

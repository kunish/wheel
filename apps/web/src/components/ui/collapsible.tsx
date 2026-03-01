import { createContext, use, useState } from "react"
import { cn } from "@/lib/utils"

interface CollapsibleContextValue {
  open: boolean
  toggle: () => void
}

const CollapsibleContext = createContext<CollapsibleContextValue>({
  open: false,
  toggle: () => {},
})

interface CollapsibleProps {
  open?: boolean
  onOpenChange?: (open: boolean) => void
  children: React.ReactNode
  className?: string
}

function Collapsible({
  open: controlledOpen,
  onOpenChange,
  children,
  className,
}: CollapsibleProps) {
  const [internalOpen, setInternalOpen] = useState(false)
  const isControlled = controlledOpen !== undefined
  const open = isControlled ? controlledOpen : internalOpen

  const toggle = () => {
    const next = !open
    if (isControlled) {
      onOpenChange?.(next)
    } else {
      setInternalOpen(next)
    }
  }

  return (
    <CollapsibleContext value={{ open, toggle }}>
      <div className={className}>{children}</div>
    </CollapsibleContext>
  )
}

interface CollapsibleTriggerProps {
  asChild?: boolean
  children: React.ReactElement<{ onClick?: () => void }>
}

function CollapsibleTrigger({ asChild, children }: CollapsibleTriggerProps) {
  const { toggle } = use(CollapsibleContext)

  if (asChild && children) {
    return (
      <button type="button" onClick={toggle} className="inline w-full text-left">
        {children}
      </button>
    )
  }

  return (
    <button type="button" onClick={toggle}>
      {children}
    </button>
  )
}

function CollapsibleContent({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  const { open } = use(CollapsibleContext)

  if (!open) return null

  return <div className={cn(className)}>{children}</div>
}

export { Collapsible, CollapsibleContent, CollapsibleTrigger }

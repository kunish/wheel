import * as React from "react"

import { cn } from "@/lib/utils"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "placeholder:text-muted-foreground aria-invalid:border-destructive border-border bg-background flex field-sizing-content min-h-16 w-full rounded-md border-2 px-3 py-2 text-base shadow-[2px_2px_0_var(--nb-shadow)] transition-all outline-none focus-visible:shadow-[4px_4px_0_var(--nb-shadow)] focus-visible:ring-0 disabled:cursor-not-allowed disabled:opacity-50 md:text-sm",
        className,
      )}
      {...props}
    />
  )
}

export { Textarea }

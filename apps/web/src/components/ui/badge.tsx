import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"
import { Slot } from "radix-ui"
import * as React from "react"

import { cn } from "@/lib/utils"

const badgeVariants = cva(
  "inline-flex items-center justify-center border-2 border-border px-2.5 py-0.5 text-xs font-bold w-fit whitespace-nowrap shrink-0 [&>svg]:size-3 gap-1 [&>svg]:pointer-events-none transition-colors overflow-hidden rounded-md shadow-[2px_2px_0_var(--nb-shadow)]",
  {
    variants: {
      variant: {
        default: "bg-primary text-primary-foreground",
        secondary: "bg-secondary text-secondary-foreground",
        destructive: "bg-destructive text-white",
        outline: "bg-background text-foreground",
        ghost: "border-transparent shadow-none bg-muted text-muted-foreground",
        lime: "bg-nb-lime text-foreground",
        pink: "bg-nb-pink text-foreground",
        sky: "bg-nb-sky text-foreground",
        orange: "bg-nb-orange text-foreground",
        lavender: "bg-nb-lavender text-foreground",
        link: "text-primary underline-offset-4 [a&]:hover:underline border-0 shadow-none",
      },
    },
    defaultVariants: {
      variant: "default",
    },
  },
)

function Badge({
  className,
  variant = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"span"> & VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "span"

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      className={cn(badgeVariants({ variant }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }

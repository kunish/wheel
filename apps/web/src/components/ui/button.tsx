import type { VariantProps } from "class-variance-authority"
import { cva } from "class-variance-authority"
import { Slot } from "radix-ui"
import * as React from "react"

import { cn } from "@/lib/utils"

const buttonVariants = cva(
  "inline-flex items-center justify-center gap-2 whitespace-nowrap text-sm font-bold transition-all disabled:pointer-events-none disabled:opacity-50 [&_svg]:pointer-events-none [&_svg:not([class*='size-'])]:size-4 shrink-0 [&_svg]:shrink-0 outline-none focus-visible:ring-2 focus-visible:ring-ring aria-invalid:ring-destructive/20 aria-invalid:border-destructive cursor-pointer active:translate-x-[2px] active:translate-y-[2px] active:shadow-none",
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground border-2 border-border shadow-[3px_3px_0_var(--nb-shadow)] hover:shadow-[1px_1px_0_var(--nb-shadow)] hover:translate-x-[2px] hover:translate-y-[2px]",
        destructive:
          "bg-destructive text-white border-2 border-border shadow-[3px_3px_0_var(--nb-shadow)] hover:shadow-[1px_1px_0_var(--nb-shadow)] hover:translate-x-[2px] hover:translate-y-[2px]",
        outline:
          "border-2 border-border bg-background shadow-[3px_3px_0_var(--nb-shadow)] hover:shadow-[1px_1px_0_var(--nb-shadow)] hover:translate-x-[2px] hover:translate-y-[2px] hover:bg-accent hover:text-accent-foreground",
        secondary:
          "bg-secondary text-secondary-foreground border-2 border-border shadow-[3px_3px_0_var(--nb-shadow)] hover:shadow-[1px_1px_0_var(--nb-shadow)] hover:translate-x-[2px] hover:translate-y-[2px]",
        ghost: "hover:bg-accent/30 hover:text-accent-foreground border-2 border-transparent",
        link: "text-primary underline-offset-4 hover:underline border-0",
      },
      size: {
        default: "h-10 px-4 py-2 rounded-md has-[>svg]:px-3",
        xs: "h-6 gap-1 rounded-sm px-2 text-xs has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-8 rounded-md gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-11 rounded-md px-6 has-[>svg]:px-4 text-base",
        icon: "size-10 rounded-md",
        "icon-xs": "size-6 rounded-sm [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-8 rounded-md",
        "icon-lg": "size-10 rounded-md",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  },
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Button, buttonVariants }

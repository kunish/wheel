import { GearSpinner } from "@/components/ui/gear-spinner"
import { cn } from "@/lib/utils"

interface SkeletonProps extends React.ComponentProps<"div"> {
  variant?: "default" | "circle" | "gear"
}

function Skeleton({ className, variant = "default", ...props }: SkeletonProps) {
  if (variant === "gear") {
    return (
      <div
        data-slot="skeleton"
        className={cn("flex items-center justify-center", className)}
        {...props}
      >
        <GearSpinner size="sm" speed="slow" className="text-muted-foreground/50" />
      </div>
    )
  }

  return (
    <div
      data-slot="skeleton"
      className={cn(
        "animate-gear-shimmer rounded-md",
        variant === "circle" && "rounded-full",
        className,
      )}
      {...props}
    />
  )
}

export { Skeleton }

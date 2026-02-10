"use client"

import dynamic from "next/dynamic"
import { Skeleton } from "@/components/ui/skeleton"

function ChartFallback() {
  return <Skeleton className="h-[160px] w-full rounded-md" />
}

export const LazyAreaChart = dynamic(
  () => import("recharts").then((mod) => ({ default: mod.AreaChart })),
  { ssr: false, loading: ChartFallback },
)

export const LazyResponsiveContainer = dynamic(
  () => import("recharts").then((mod) => ({ default: mod.ResponsiveContainer })),
  { ssr: false, loading: ChartFallback },
)

export const LazyXAxis = dynamic(() => import("recharts").then((mod) => ({ default: mod.XAxis })), {
  ssr: false,
})

export const LazyYAxis = dynamic(() => import("recharts").then((mod) => ({ default: mod.YAxis })), {
  ssr: false,
})

export const LazyArea = dynamic(() => import("recharts").then((mod) => ({ default: mod.Area })), {
  ssr: false,
})

export const LazyRechartsTooltip = dynamic(
  () => import("recharts").then((mod) => ({ default: mod.Tooltip })),
  { ssr: false },
)

export const LazyCartesianGrid = dynamic(
  () => import("recharts").then((mod) => ({ default: mod.CartesianGrid })),
  { ssr: false },
)

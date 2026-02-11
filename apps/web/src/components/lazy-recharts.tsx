import { lazy } from "react"

export const LazyAreaChart = lazy(() =>
  import("recharts").then((mod) => ({ default: mod.AreaChart })),
)

export const LazyResponsiveContainer = lazy(() =>
  import("recharts").then((mod) => ({ default: mod.ResponsiveContainer })),
)

export const LazyXAxis = lazy(() => import("recharts").then((mod) => ({ default: mod.XAxis })))

export const LazyYAxis = lazy(() => import("recharts").then((mod) => ({ default: mod.YAxis })))

export const LazyArea = lazy(() => import("recharts").then((mod) => ({ default: mod.Area })))

export const LazyRechartsTooltip = lazy(() =>
  import("recharts").then((mod) => ({ default: mod.Tooltip })),
)

export const LazyCartesianGrid = lazy(() =>
  import("recharts").then((mod) => ({ default: mod.CartesianGrid })),
)

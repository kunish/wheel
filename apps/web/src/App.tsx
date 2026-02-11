import { Suspense } from "react"
import { GearSpinner } from "@/components/ui/gear-spinner"
import { Toaster } from "@/components/ui/sonner"
import { QueryProvider } from "./components/query-provider"
import { ThemeProvider } from "./components/theme-provider"
import { AppRouter } from "./routes"

function LoadingFallback() {
  return (
    <div className="flex h-screen flex-col items-center justify-center gap-3">
      <GearSpinner size="lg" className="text-muted-foreground" />
      <span className="text-muted-foreground text-sm font-bold">Loading</span>
    </div>
  )
}

export function App() {
  return (
    <ThemeProvider attribute="class" defaultTheme="system" enableSystem disableTransitionOnChange>
      <QueryProvider>
        <Suspense fallback={<LoadingFallback />}>
          <AppRouter />
        </Suspense>
        <Toaster />
      </QueryProvider>
    </ThemeProvider>
  )
}

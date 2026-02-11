import { Suspense } from "react"
import { Toaster } from "@/components/ui/sonner"
import { QueryProvider } from "./components/query-provider"
import { ThemeProvider } from "./components/theme-provider"
import { AppRouter } from "./routes"

function LoadingFallback() {
  return (
    <div className="flex h-screen items-center justify-center">
      <div className="text-muted-foreground h-8 w-8 animate-spin rounded-full border-4 border-current border-t-transparent" />
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

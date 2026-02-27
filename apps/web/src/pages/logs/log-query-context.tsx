import { createContext, use } from "react"
import { useLogQuery } from "@/hooks/use-log-query"

type LogQueryContextValue = ReturnType<typeof useLogQuery>

const LogQueryContext = createContext<LogQueryContextValue | null>(null)

export function LogQueryProvider({ children }: { children: React.ReactNode }) {
  const value = useLogQuery()
  return <LogQueryContext value={value}>{children}</LogQueryContext>
}

export function useLogQueryContext(): LogQueryContextValue {
  const ctx = use(LogQueryContext)
  if (ctx === null) {
    throw new Error("useLogQueryContext must be used within a LogQueryProvider")
  }
  return ctx
}

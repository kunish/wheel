import { useQueryClient } from "@tanstack/react-query"
import { Navigate, Outlet } from "react-router"
import { AppLayout } from "@/components/app-layout"
import { useStatsWebSocket } from "@/hooks/use-stats-ws"
import { useAuthStore } from "@/lib/store/auth"

export function ProtectedLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const queryClient = useQueryClient()
  useStatsWebSocket(queryClient)

  if (!isAuthenticated()) {
    return <Navigate to="/login" replace />
  }

  return (
    <AppLayout>
      <Outlet />
    </AppLayout>
  )
}

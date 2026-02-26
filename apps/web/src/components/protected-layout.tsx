import { useQueryClient } from "@tanstack/react-query"
import { AnimatePresence, motion } from "motion/react"
import { useRef } from "react"
import { Navigate, Outlet, useLocation } from "react-router"
import { AppLayout } from "@/components/app-layout"
import { useStatsWebSocket } from "@/hooks/use-stats-ws"
import { useAuthStore } from "@/lib/store/auth"

/** Extract the first path segment as a stable transition key,
 *  so query params or nested route changes don't trigger re-animation. */
function useRouteKey() {
  const { pathname } = useLocation()
  return `/${pathname.split("/")[1] || ""}`
}

export function ProtectedLayout() {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated())
  const queryClient = useQueryClient()
  useStatsWebSocket(queryClient)
  const routeKey = useRouteKey()
  const prevKeyRef = useRef(routeKey)

  // Determine scroll direction: entering route index vs previous
  const navOrder = ["/dashboard", "/model", "/mcp", "/logs", "/settings"]
  const prevIdx = navOrder.indexOf(prevKeyRef.current)
  const currIdx = navOrder.indexOf(routeKey)
  const direction = currIdx >= prevIdx ? 1 : -1

  // Update ref after computing direction
  if (prevKeyRef.current !== routeKey) {
    prevKeyRef.current = routeKey
  }

  if (!isAuthenticated) {
    return <Navigate to="/login" replace />
  }

  return (
    <AppLayout>
      <AnimatePresence mode="popLayout" initial={false}>
        <motion.div
          key={routeKey}
          className="flex min-h-0 flex-1 flex-col"
          initial={{ opacity: 0, y: direction * 8 }}
          animate={{ opacity: 1, y: 0 }}
          exit={{ opacity: 0, y: direction * -4 }}
          transition={{ duration: 0.15, ease: [0.25, 1, 0.5, 1] }}
          style={{ willChange: "opacity, transform" }}
        >
          <Outlet />
        </motion.div>
      </AnimatePresence>
    </AppLayout>
  )
}

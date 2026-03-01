import { lazy } from "react"
import { BrowserRouter, HashRouter, Navigate, Route, Routes } from "react-router"
import { ProtectedLayout } from "./components/protected-layout"
import { queryClient } from "./components/query-provider"
import {
  getChannelStats,
  getDailyStats,
  getHourlyStats,
  getModelStats,
  getTotalStats,
} from "./lib/api-client"

const LoginPage = lazy(() => import("./pages/login"))
const KeysPage = lazy(() => import("./pages/keys"))
const DashboardPage = lazy(() => {
  // Prefetch all dashboard data in parallel with chunk loading
  const opts = { staleTime: 30 * 1000 }
  queryClient.prefetchQuery({ queryKey: ["stats", "total"], queryFn: getTotalStats, ...opts })
  queryClient.prefetchQuery({ queryKey: ["stats", "daily"], queryFn: getDailyStats, ...opts })
  queryClient.prefetchQuery({
    queryKey: ["stats", "hourly"],
    queryFn: () => getHourlyStats(),
    ...opts,
  })
  queryClient.prefetchQuery({ queryKey: ["stats", "channel"], queryFn: getChannelStats, ...opts })
  queryClient.prefetchQuery({ queryKey: ["stats", "model"], queryFn: getModelStats, ...opts })
  return import("./pages/dashboard")
})
const ModelPage = lazy(() => import("./pages/model"))
const GroupsPage = lazy(() => import("./pages/groups"))
const LogsPage = lazy(() => import("./pages/logs"))
const SettingsPage = lazy(() => import("./pages/settings"))
const MCPPage = lazy(() => import("./pages/mcp"))
const ModelLimitsPage = lazy(() => import("./pages/model-limits"))
const PlaygroundPage = lazy(() => import("./pages/playground"))
const UsagePage = lazy(() => import("./pages/usage"))
const BudgetsPage = lazy(() => import("./pages/budgets"))
const GuardrailsPage = lazy(() => import("./pages/guardrails"))
const ApiReferencePage = lazy(() => import("./pages/api-reference"))
const TagsPage = lazy(() => import("./pages/tags"))

const Router = import.meta.env.VITE_HASH_ROUTER === "true" ? HashRouter : BrowserRouter

export function AppRouter() {
  return (
    <Router>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedLayout />}>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/model" element={<ModelPage />} />
          <Route path="/playground" element={<PlaygroundPage />} />
          <Route path="/channels" element={<Navigate to="/model" replace />} />
          <Route path="/groups" element={<GroupsPage />} />
          <Route path="/keys" element={<KeysPage />} />
          <Route path="/logs" element={<LogsPage />} />
          <Route path="/usage" element={<UsagePage />} />
          <Route path="/budgets" element={<BudgetsPage />} />
          <Route path="/model-limits" element={<ModelLimitsPage />} />
          <Route path="/guardrails" element={<GuardrailsPage />} />
          <Route path="/tags" element={<TagsPage />} />
          <Route path="/api-reference" element={<ApiReferencePage />} />
          <Route path="/prices" element={<Navigate to="/model" replace />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/mcp" element={<MCPPage />} />
          <Route path="/apikeys" element={<Navigate to="/keys" replace />} />
        </Route>
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </Router>
  )
}

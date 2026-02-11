import { lazy } from "react"
import { BrowserRouter, Navigate, Route, Routes } from "react-router"
import { ProtectedLayout } from "./components/protected-layout"

const LoginPage = lazy(() => import("./pages/login"))
const DashboardPage = lazy(() => import("./pages/dashboard"))
const ChannelsPage = lazy(() => import("./pages/channels"))
const GroupsPage = lazy(() => import("./pages/groups"))
const LogsPage = lazy(() => import("./pages/logs"))
const PricesPage = lazy(() => import("./pages/prices"))
const SettingsPage = lazy(() => import("./pages/settings"))

export function AppRouter() {
  return (
    <BrowserRouter>
      <Routes>
        <Route path="/" element={<Navigate to="/dashboard" replace />} />
        <Route path="/login" element={<LoginPage />} />
        <Route element={<ProtectedLayout />}>
          <Route path="/dashboard" element={<DashboardPage />} />
          <Route path="/channels" element={<ChannelsPage />} />
          <Route path="/groups" element={<GroupsPage />} />
          <Route path="/logs" element={<LogsPage />} />
          <Route path="/prices" element={<PricesPage />} />
          <Route path="/settings" element={<SettingsPage />} />
          <Route path="/apikeys" element={<Navigate to="/settings" replace />} />
        </Route>
        <Route path="*" element={<Navigate to="/dashboard" replace />} />
      </Routes>
    </BrowserRouter>
  )
}

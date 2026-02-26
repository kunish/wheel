import {
  Boxes,
  ChevronLeft,
  ChevronRight,
  Ellipsis,
  FileText,
  Gauge,
  Key,
  Languages,
  LayoutDashboard,
  LogOut,
  Monitor,
  Moon,
  Plug,
  Search,
  Settings,
  Sun,
} from "lucide-react"
import { motion } from "motion/react"
import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation } from "react-router"
import { useTheme } from "@/components/theme-provider"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { useAuthStore } from "@/lib/store/auth"
import { cn } from "@/lib/utils"

const navItemDefs = [
  { href: "/dashboard", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { href: "/model", labelKey: "nav.model", icon: Boxes },
  { href: "/keys", labelKey: "nav.keys", icon: Key },
  { href: "/mcp", labelKey: "nav.mcp", icon: Plug },
  { href: "/logs", labelKey: "nav.logs", icon: FileText },
  { href: "/model-limits", labelKey: "nav.modelLimits", icon: Gauge },
  { href: "/settings", labelKey: "nav.settings", icon: Settings },
] as const

const SIDEBAR_COLLAPSED_KEY = "wheel-sidebar-collapsed"

function useMediaQuery(query: string) {
  const [matches, setMatches] = useState(() =>
    typeof window !== "undefined" ? window.matchMedia(query).matches : false,
  )
  useEffect(() => {
    const mql = window.matchMedia(query)
    const handler = (e: MediaQueryListEvent) => setMatches(e.matches)
    mql.addEventListener("change", handler)
    return () => mql.removeEventListener("change", handler)
  }, [query])
  return matches
}

// ── Desktop Sidebar ──

function DesktopSidebar() {
  const { pathname } = useLocation()
  const { t, i18n } = useTranslation()
  const { theme, setTheme } = useTheme()
  const logout = useAuthStore((s) => s.logout)
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false)
  const [collapsed, setCollapsed] = useState(
    () => localStorage.getItem(SIDEBAR_COLLAPSED_KEY) === "true",
  )

  const toggleCollapsed = useCallback(() => {
    setCollapsed((prev) => {
      const next = !prev
      localStorage.setItem(SIDEBAR_COLLAPSED_KEY, String(next))
      return next
    })
  }, [])

  return (
    <>
      <TooltipProvider delayDuration={0}>
        <aside
          className={cn(
            "bg-sidebar border-sidebar-border flex h-full shrink-0 flex-col border-r transition-[width] duration-200",
            collapsed ? "w-16" : "w-52",
          )}
        >
          {/* Logo / Brand */}
          <div className="flex h-14 items-center gap-2 px-4">
            {!collapsed && <span className="text-lg font-black tracking-tight">Wheel</span>}
            <button
              type="button"
              onClick={toggleCollapsed}
              className={cn(
                "text-muted-foreground hover:text-foreground rounded-md p-1 transition-colors",
                collapsed && "mx-auto",
              )}
              aria-label="Toggle sidebar"
            >
              {collapsed ? (
                <ChevronRight className="h-4 w-4" />
              ) : (
                <ChevronLeft className="h-4 w-4" />
              )}
            </button>
          </div>

          {/* Search shortcut */}
          <div className="px-3 pb-2">
            <button
              type="button"
              onClick={() => {
                window.dispatchEvent(new KeyboardEvent("keydown", { key: "k", metaKey: true }))
              }}
              className={cn(
                "bg-muted/50 text-muted-foreground hover:bg-muted flex w-full items-center gap-2 rounded-lg px-3 py-1.5 text-sm transition-colors",
                collapsed && "justify-center px-0",
              )}
            >
              <Search className="h-3.5 w-3.5 shrink-0" />
              {!collapsed && (
                <>
                  <span className="flex-1 text-left text-xs">{t("actions.search")}</span>
                  <kbd className="bg-background rounded px-1 py-0.5 font-mono text-[10px]">⌘K</kbd>
                </>
              )}
            </button>
          </div>

          {/* Nav items */}
          <nav className="flex flex-1 flex-col gap-0.5 px-3">
            {navItemDefs.map((item) => {
              const Icon = item.icon
              const isActive = pathname.startsWith(item.href)
              const link = (
                <Link
                  key={item.href}
                  to={item.href}
                  className={cn(
                    "relative isolate flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-all",
                    isActive
                      ? "text-sidebar-primary-foreground"
                      : "text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent",
                    collapsed && "justify-center px-0",
                  )}
                >
                  {isActive && (
                    <motion.div
                      layoutId="sidebar-active"
                      className="bg-nb-lime absolute inset-0 -z-10 rounded-lg"
                      transition={{ type: "spring", stiffness: 400, damping: 30 }}
                    />
                  )}
                  <Icon className="h-4.5 w-4.5 shrink-0" />
                  {!collapsed && <span>{t(item.labelKey)}</span>}
                </Link>
              )

              if (collapsed) {
                return (
                  <Tooltip key={item.href}>
                    <TooltipTrigger asChild>{link}</TooltipTrigger>
                    <TooltipContent side="right" sideOffset={8}>
                      {t(item.labelKey)}
                    </TooltipContent>
                  </Tooltip>
                )
              }
              return link
            })}
          </nav>

          {/* Bottom actions */}
          <div className="flex flex-col gap-1 border-t px-3 py-3">
            <DropdownMenu>
              <DropdownMenuTrigger asChild>
                <button
                  type="button"
                  className={cn(
                    "text-sidebar-foreground/60 hover:text-sidebar-foreground hover:bg-sidebar-accent flex items-center gap-3 rounded-lg px-3 py-2 text-sm font-medium transition-colors",
                    collapsed && "justify-center px-0",
                  )}
                >
                  <Ellipsis className="h-4.5 w-4.5 shrink-0" />
                  {!collapsed && <span>{t("nav.more", "More")}</span>}
                </button>
              </DropdownMenuTrigger>
              <DropdownMenuContent
                side={collapsed ? "right" : "top"}
                align="start"
                className="mb-1"
              >
                {(
                  [
                    { code: "en", label: "English" },
                    { code: "zh-CN", label: "中文" },
                  ] as const
                ).map((lang) => (
                  <DropdownMenuItem
                    key={lang.code}
                    onClick={() => i18n.changeLanguage(lang.code)}
                    className={i18n.language === lang.code ? "bg-accent/30" : ""}
                  >
                    <Languages className="size-4" />
                    {lang.label}
                  </DropdownMenuItem>
                ))}
                {(
                  [
                    { value: "light", label: t("theme.light"), icon: Sun },
                    { value: "dark", label: t("theme.dark"), icon: Moon },
                    { value: "system", label: t("theme.system"), icon: Monitor },
                  ] as const
                ).map((item) => {
                  const ThemeIcon = item.icon
                  return (
                    <DropdownMenuItem
                      key={item.value}
                      onClick={() => setTheme(item.value)}
                      className={theme === item.value ? "bg-accent/30" : ""}
                    >
                      <ThemeIcon className="size-4" />
                      {item.label}
                    </DropdownMenuItem>
                  )
                })}
                <DropdownMenuItem onClick={() => setShowLogoutConfirm(true)}>
                  <LogOut className="size-4" />
                  {t("logout.button")}
                </DropdownMenuItem>
              </DropdownMenuContent>
            </DropdownMenu>
          </div>
        </aside>
      </TooltipProvider>

      <AlertDialog open={showLogoutConfirm} onOpenChange={setShowLogoutConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("logout.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("logout.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("logout.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                logout()
                window.location.href = "/login"
              }}
            >
              {t("logout.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

// ── Mobile Bottom Nav ──

function BottomNav() {
  const { pathname } = useLocation()
  const { t, i18n } = useTranslation()
  const { theme, setTheme } = useTheme()
  const logout = useAuthStore((s) => s.logout)
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false)

  // Show only 5 items on mobile bottom nav (skip keys)
  const mobileNavItems = navItemDefs.filter((item) => item.href !== "/keys")

  return (
    <>
      <nav className="bg-sidebar border-sidebar-border fixed inset-x-0 bottom-0 z-50 flex h-16 items-stretch border-t-2">
        {mobileNavItems.map((item) => {
          const Icon = item.icon
          const isActive = pathname.startsWith(item.href)
          return (
            <Link
              key={item.href}
              to={item.href}
              className={cn(
                "relative isolate flex flex-1 flex-col items-center justify-center gap-0.5 transition-all",
                isActive ? "text-sidebar-primary-foreground" : "text-sidebar-foreground/50",
              )}
            >
              {isActive && (
                <motion.div
                  layoutId="bottom-nav-active"
                  className="bg-nb-lime absolute inset-x-1 inset-y-1.5 -z-10 rounded-lg"
                  transition={{ type: "spring", stiffness: 400, damping: 30 }}
                />
              )}
              <Icon className="size-5" />
              <span className="text-[10px] leading-none font-bold">{t(item.labelKey)}</span>
            </Link>
          )
        })}

        {/* More menu */}
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              type="button"
              className="text-sidebar-foreground/50 flex flex-1 flex-col items-center justify-center gap-0.5"
            >
              <Ellipsis className="size-5" />
              <span className="text-[10px] leading-none font-bold">{t("nav.more", "More")}</span>
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent side="top" align="end" className="mb-2">
            {(
              [
                { code: "en", label: "English" },
                { code: "zh-CN", label: "中文" },
              ] as const
            ).map((lang) => (
              <DropdownMenuItem
                key={lang.code}
                onClick={() => i18n.changeLanguage(lang.code)}
                className={i18n.language === lang.code ? "bg-accent/30" : ""}
              >
                <Languages className="size-4" />
                {lang.label}
              </DropdownMenuItem>
            ))}
            {(
              [
                { value: "light", label: t("theme.light"), icon: Sun },
                { value: "dark", label: t("theme.dark"), icon: Moon },
                { value: "system", label: t("theme.system"), icon: Monitor },
              ] as const
            ).map((item) => {
              const ThemeIcon = item.icon
              return (
                <DropdownMenuItem
                  key={item.value}
                  onClick={() => setTheme(item.value)}
                  className={theme === item.value ? "bg-accent/30" : ""}
                >
                  <ThemeIcon className="size-4" />
                  {item.label}
                </DropdownMenuItem>
              )
            })}
            <DropdownMenuItem onClick={() => setShowLogoutConfirm(true)}>
              <LogOut className="size-4" />
              {t("logout.button")}
            </DropdownMenuItem>
          </DropdownMenuContent>
        </DropdownMenu>
      </nav>

      <AlertDialog open={showLogoutConfirm} onOpenChange={setShowLogoutConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t("logout.title")}</AlertDialogTitle>
            <AlertDialogDescription>{t("logout.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("logout.cancel")}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                logout()
                window.location.href = "/login"
              }}
            >
              {t("logout.confirm")}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  )
}

// ── App Layout ──

export function AppLayout({ children }: { children: React.ReactNode }) {
  const isDesktop = useMediaQuery("(min-width: 1024px)")

  if (isDesktop) {
    return (
      <div className="flex h-screen">
        <DesktopSidebar />
        <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-6">{children}</main>
      </div>
    )
  }

  return (
    <div className="flex h-screen flex-col">
      <main className="flex min-h-0 flex-1 flex-col overflow-hidden p-4 pb-20">{children}</main>
      <BottomNav />
    </div>
  )
}

import {
  Boxes,
  Ellipsis,
  FileText,
  Languages,
  LayoutDashboard,
  LogOut,
  Moon,
  Settings,
  Sun,
} from "lucide-react"
import { motion } from "motion/react"
import { useTheme } from "next-themes"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { Link, useLocation } from "react-router"
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
import { useAuthStore } from "@/lib/store/auth"
import { cn } from "@/lib/utils"

const navItemDefs = [
  { href: "/dashboard", labelKey: "nav.dashboard", icon: LayoutDashboard },
  { href: "/model", labelKey: "nav.model", icon: Boxes },
  { href: "/logs", labelKey: "nav.logs", icon: FileText },
  { href: "/settings", labelKey: "nav.settings", icon: Settings },
] as const

function BottomNav() {
  const { pathname } = useLocation()
  const { t, i18n } = useTranslation()
  const { theme, setTheme } = useTheme()
  const logout = useAuthStore((s) => s.logout)
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false)

  return (
    <>
      <nav className="bg-sidebar border-sidebar-border fixed inset-x-0 bottom-0 z-50 flex h-16 items-stretch border-t-2">
        {navItemDefs.map((item) => {
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
            <button className="text-sidebar-foreground/50 flex flex-1 flex-col items-center justify-center gap-0.5">
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
            <DropdownMenuItem onClick={() => setTheme(theme === "dark" ? "light" : "dark")}>
              {theme === "dark" ? <Sun className="size-4" /> : <Moon className="size-4" />}
              {theme === "dark" ? t("theme.light") : t("theme.dark")}
            </DropdownMenuItem>
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

export function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen flex-col">
      {/* Main content */}
      <main className="flex flex-1 flex-col overflow-auto p-4 pb-20 lg:p-6 lg:pb-20">
        {children}
      </main>

      {/* Bottom nav — all screen sizes */}
      <BottomNav />
    </div>
  )
}

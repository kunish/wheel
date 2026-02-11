import {
  DollarSign,
  FileText,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  Radio,
  Settings,
  Sun,
} from "lucide-react"
import { useTheme } from "next-themes"
import { useState } from "react"
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
import { Button } from "@/components/ui/button"
import { ScrollArea } from "@/components/ui/scroll-area"
import { Sheet, SheetContent, SheetTitle, SheetTrigger } from "@/components/ui/sheet"
import { useAuthStore } from "@/lib/store/auth"
import { cn } from "@/lib/utils"

const navItems = [
  { href: "/dashboard", label: "Dashboard", icon: LayoutDashboard },
  { href: "/channels", label: "Channels & Groups", icon: Radio },
  { href: "/prices", label: "Prices", icon: DollarSign },
  { href: "/logs", label: "Logs", icon: FileText },
  { href: "/settings", label: "Settings", icon: Settings },
]

function NavContent({ onNavigate }: { onNavigate?: () => void }) {
  const { pathname } = useLocation()

  return (
    <div className="flex h-full flex-col">
      {/* Logo */}
      <div className="p-5">
        <div className="flex items-center gap-2.5">
          <div className="bg-nb-lime border-sidebar-foreground flex size-9 items-center justify-center rounded-md border-2">
            <svg
              className="size-5"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
            >
              <circle cx="12" cy="12" r="9" />
              <circle cx="12" cy="12" r="3" />
              <line x1="12" y1="3" x2="12" y2="9" />
              <line x1="12" y1="15" x2="12" y2="21" />
              <line x1="3" y1="12" x2="9" y2="12" />
              <line x1="15" y1="12" x2="21" y2="12" />
              <line x1="5.64" y1="5.64" x2="9.88" y2="9.88" />
              <line x1="14.12" y1="14.12" x2="18.36" y2="18.36" />
              <line x1="18.36" y1="5.64" x2="14.12" y2="9.88" />
              <line x1="9.88" y1="14.12" x2="5.64" y2="18.36" />
            </svg>
          </div>
          <div>
            <h1 className="text-sidebar-foreground text-base font-bold tracking-tight">Wheel</h1>
            <p className="text-sidebar-foreground/50 text-[10px] font-medium tracking-widest uppercase">
              LLM Gateway
            </p>
          </div>
        </div>
      </div>

      {/* Divider */}
      <div className="border-sidebar-border mx-4 border-t-2" />

      {/* Nav */}
      <ScrollArea className="flex-1 px-3 py-4">
        <nav className="flex flex-col gap-1">
          {navItems.map((item) => {
            const Icon = item.icon
            const isActive = pathname.startsWith(item.href)
            return (
              <Link
                key={item.href}
                to={item.href}
                onClick={onNavigate}
                className={cn(
                  "flex items-center gap-3 rounded-md px-3 py-2.5 text-sm font-bold transition-all",
                  isActive
                    ? "bg-nb-lime text-sidebar-primary-foreground border-sidebar-foreground border-2 shadow-[2px_2px_0_rgba(255,255,255,0.15)]"
                    : "text-sidebar-foreground/70 hover:bg-sidebar-accent hover:text-sidebar-foreground border-2 border-transparent",
                )}
              >
                <Icon className="size-4" />
                {item.label}
              </Link>
            )
          })}
        </nav>
      </ScrollArea>

      {/* Footer */}
      <div className="border-sidebar-border mx-4 border-t-2" />
      <div className="p-3">
        <div className="text-sidebar-foreground/40 px-3 py-2 text-[10px] font-medium tracking-widest uppercase">
          Wheel
        </div>
      </div>
    </div>
  )
}

function TopBar() {
  const { theme, setTheme } = useTheme()
  const logout = useAuthStore((s) => s.logout)
  const [showLogoutConfirm, setShowLogoutConfirm] = useState(false)

  return (
    <header className="border-border bg-background flex h-14 items-center gap-3 border-b-2 px-4 lg:px-6">
      {/* Mobile menu */}
      <Sheet>
        <SheetTrigger asChild>
          <Button
            variant="ghost"
            size="icon"
            className="lg:hidden"
            aria-label="Open navigation menu"
          >
            <Menu className="size-5" />
          </Button>
        </SheetTrigger>
        <SheetContent
          side="left"
          className="bg-sidebar border-sidebar-border w-64 border-r-2 p-0"
          showCloseButton={false}
        >
          <SheetTitle className="sr-only">Navigation</SheetTitle>
          <NavContent />
        </SheetContent>
      </Sheet>

      <div className="flex-1" />

      <Button
        variant="ghost"
        size="icon-sm"
        aria-label={theme === "dark" ? "Switch to light mode" : "Switch to dark mode"}
        onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
      >
        <Sun className="size-4 scale-100 rotate-0 transition-all dark:scale-0 dark:-rotate-90" />
        <Moon className="absolute size-4 scale-0 rotate-90 transition-all dark:scale-100 dark:rotate-0" />
      </Button>

      <Button
        variant="ghost"
        size="icon-sm"
        aria-label="Logout"
        onClick={() => setShowLogoutConfirm(true)}
      >
        <LogOut className="size-4" />
      </Button>

      <AlertDialog open={showLogoutConfirm} onOpenChange={setShowLogoutConfirm}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>Are you sure you want to logout?</AlertDialogTitle>
            <AlertDialogDescription>
              You will need to sign in again to access the dashboard.
            </AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>Cancel</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                logout()
                window.location.href = "/login"
              }}
            >
              Logout
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </header>
  )
}

export function AppLayout({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex h-screen">
      {/* Desktop sidebar — dark panel */}
      <aside className="bg-sidebar border-sidebar-border hidden border-r-2 lg:flex lg:w-60 lg:flex-col">
        <NavContent />
      </aside>

      {/* Main content */}
      <div className="flex flex-1 flex-col overflow-hidden">
        <TopBar />
        <main className="flex-1 overflow-auto p-4 lg:p-6">{children}</main>
      </div>
    </div>
  )
}

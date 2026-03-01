import {
  BarChart3,
  BookOpen,
  Boxes,
  DollarSign,
  FileText,
  Key,
  LayoutDashboard,
  Moon,
  Plug,
  Search,
  Settings,
  Shield,
  Sun,
  Tags,
  Terminal,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { useTheme } from "@/components/theme-provider"
import { cn } from "@/lib/utils"

interface CommandItem {
  id: string
  label: string
  icon: React.ElementType
  action: () => void
  keywords?: string[]
  group: string
}

export function CommandPalette() {
  const [open, setOpen] = useState(false)
  const [search, setSearch] = useState("")
  const [selectedIndex, setSelectedIndex] = useState(0)
  const navigate = useNavigate()
  const { t } = useTranslation()
  const { setTheme } = useTheme()

  const items = useMemo<CommandItem[]>(
    () => [
      {
        id: "nav-dashboard",
        label: t("nav.dashboard"),
        icon: LayoutDashboard,
        action: () => navigate("/dashboard"),
        keywords: ["home", "overview", "仪表盘"],
        group: "Navigation",
      },
      {
        id: "nav-model",
        label: t("nav.model"),
        icon: Boxes,
        action: () => navigate("/model"),
        keywords: ["channel", "group", "模型", "渠道"],
        group: "Navigation",
      },
      {
        id: "nav-keys",
        label: "API Keys",
        icon: Key,
        action: () => navigate("/keys"),
        keywords: ["token", "secret", "密钥"],
        group: "Navigation",
      },
      {
        id: "nav-mcp",
        label: t("nav.mcp"),
        icon: Plug,
        action: () => navigate("/mcp"),
        keywords: ["tool", "server"],
        group: "Navigation",
      },
      {
        id: "nav-logs",
        label: t("nav.logs"),
        icon: FileText,
        action: () => navigate("/logs"),
        keywords: ["request", "history", "日志"],
        group: "Navigation",
      },
      {
        id: "nav-playground",
        label: t("nav.playground"),
        icon: Terminal,
        action: () => navigate("/playground"),
        keywords: ["test", "debug", "chat", "调试", "测试"],
        group: "Navigation",
      },
      {
        id: "nav-usage",
        label: t("nav.usage"),
        icon: BarChart3,
        action: () => navigate("/usage"),
        keywords: ["analytics", "stats", "cost", "用量", "统计"],
        group: "Navigation",
      },
      {
        id: "nav-budgets",
        label: t("nav.budgets"),
        icon: DollarSign,
        action: () => navigate("/budgets"),
        keywords: ["budget", "limit", "spending", "预算"],
        group: "Navigation",
      },
      {
        id: "nav-guardrails",
        label: t("nav.guardrails"),
        icon: Shield,
        action: () => navigate("/guardrails"),
        keywords: ["safety", "filter", "content", "安全", "护栏"],
        group: "Navigation",
      },
      {
        id: "nav-tags",
        label: t("nav.tags"),
        icon: Tags,
        action: () => navigate("/tags"),
        keywords: ["tag", "label", "category", "标签"],
        group: "Navigation",
      },
      {
        id: "nav-api-reference",
        label: t("nav.apiReference"),
        icon: BookOpen,
        action: () => navigate("/api-reference"),
        keywords: ["api", "openapi", "docs", "reference", "API文档"],
        group: "Navigation",
      },
      {
        id: "nav-audit",
        label: "Audit Logs",
        icon: Shield,
        action: () => navigate("/logs?tab=audit"),
        keywords: ["audit", "security", "审计"],
        group: "Navigation",
      },
      {
        id: "nav-settings",
        label: t("nav.settings"),
        icon: Settings,
        action: () => navigate("/settings"),
        keywords: ["config", "设置"],
        group: "Navigation",
      },
      {
        id: "theme-light",
        label: t("theme.light"),
        icon: Sun,
        action: () => setTheme("light"),
        keywords: ["theme", "亮色"],
        group: "Theme",
      },
      {
        id: "theme-dark",
        label: t("theme.dark"),
        icon: Moon,
        action: () => setTheme("dark"),
        keywords: ["theme", "暗色"],
        group: "Theme",
      },
    ],
    [t, navigate, setTheme],
  )

  const filtered = useMemo(() => {
    if (!search) return items
    const q = search.toLowerCase()
    return items.filter(
      (item) =>
        item.label.toLowerCase().includes(q) ||
        item.keywords?.some((k) => k.toLowerCase().includes(q)),
    )
  }, [items, search])

  const grouped = useMemo(() => {
    const map = new Map<string, CommandItem[]>()
    for (const item of filtered) {
      const group = map.get(item.group) ?? []
      group.push(item)
      map.set(item.group, group)
    }
    return map
  }, [filtered])

  const flatItems = useMemo(() => filtered, [filtered])

  const execute = useCallback((item: CommandItem) => {
    item.action()
    setOpen(false)
    setSearch("")
  }, [])

  // Keyboard shortcut to open
  useEffect(() => {
    function handleKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key === "k") {
        e.preventDefault()
        setOpen((prev) => !prev)
        setSearch("")
        setSelectedIndex(0)
      }
      if (e.key === "Escape") {
        setOpen(false)
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [])

  // Arrow key navigation
  useEffect(() => {
    if (!open) return
    function handleKeyDown(e: KeyboardEvent) {
      if (e.key === "ArrowDown") {
        e.preventDefault()
        setSelectedIndex((i) => Math.min(i + 1, flatItems.length - 1))
      } else if (e.key === "ArrowUp") {
        e.preventDefault()
        setSelectedIndex((i) => Math.max(i - 1, 0))
      } else if (e.key === "Enter") {
        e.preventDefault()
        if (flatItems[selectedIndex]) execute(flatItems[selectedIndex])
      }
    }
    window.addEventListener("keydown", handleKeyDown)
    return () => window.removeEventListener("keydown", handleKeyDown)
  }, [open, flatItems, selectedIndex, execute])

  // Reset selection when search changes
  useEffect(() => {
    setSelectedIndex(0)
  }, [search])

  if (!open) return null

  return (
    <div className="fixed inset-0 z-[100]" role="dialog" aria-modal="true">
      {/* Backdrop */}
      <button
        type="button"
        className="fixed inset-0 bg-black/50"
        onClick={() => setOpen(false)}
        aria-label="Close command palette"
      />

      {/* Panel */}
      <div className="fixed top-[20%] left-1/2 w-full max-w-lg -translate-x-1/2">
        <div className="bg-popover text-popover-foreground mx-4 overflow-hidden rounded-xl border shadow-2xl">
          {/* Search input */}
          <div className="flex items-center gap-2 border-b px-4 py-3">
            <Search className="text-muted-foreground h-4 w-4 shrink-0" />
            <input
              ref={(el) => el?.focus()}
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              placeholder="Type a command or search..."
              className="placeholder:text-muted-foreground flex-1 bg-transparent text-sm outline-none"
            />
            <kbd className="bg-muted text-muted-foreground rounded px-1.5 py-0.5 font-mono text-[10px]">
              ESC
            </kbd>
          </div>

          {/* Results */}
          <div className="max-h-72 overflow-y-auto p-2">
            {flatItems.length === 0 ? (
              <p className="text-muted-foreground py-6 text-center text-sm">No results found.</p>
            ) : (
              Array.from(grouped.entries()).map(([group, groupItems]) => (
                <div key={group}>
                  <p className="text-muted-foreground px-2 py-1.5 text-[10px] font-bold tracking-wider uppercase">
                    {group}
                  </p>
                  {groupItems.map((item) => {
                    const idx = flatItems.indexOf(item)
                    const Icon = item.icon
                    return (
                      <button
                        type="button"
                        key={item.id}
                        onClick={() => execute(item)}
                        onMouseEnter={() => setSelectedIndex(idx)}
                        className={cn(
                          "flex w-full items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
                          idx === selectedIndex
                            ? "bg-accent text-accent-foreground"
                            : "text-foreground",
                        )}
                      >
                        <Icon className="h-4 w-4 shrink-0 opacity-60" />
                        {item.label}
                      </button>
                    )
                  })}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    </div>
  )
}

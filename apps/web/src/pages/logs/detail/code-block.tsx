import { Check, ChevronDown, Copy, Search, X } from "lucide-react"
import * as React from "react"
import { lazy, Suspense, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { useTheme } from "@/components/theme-provider"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Skeleton } from "@/components/ui/skeleton"
import { countMatches } from "../log-filters"
import { repairTruncatedJson } from "./message-parsers"

// --- Dynamic imports: heavy libs only needed inside the detail panel ---
export const LazyJsonView = lazy(() =>
  Promise.all([
    import("@uiw/react-json-view"),
    import("@uiw/react-json-view/githubDark"),
    import("@uiw/react-json-view/githubLight"),
  ]).then(([mod, dark, light]) => {
    const JsonView = mod.default
    function LazyJsonViewInner(props: React.ComponentProps<typeof JsonView> & { isDark: boolean }) {
      const { isDark, style, ...rest } = props
      return (
        <JsonView
          {...rest}
          style={{
            ...(isDark ? dark.githubDarkTheme : light.githubLightTheme),
            ...style,
          }}
        />
      )
    }
    LazyJsonViewInner.displayName = "LazyJsonView"
    return { default: LazyJsonViewInner }
  }),
)

export const LazyMarkdown = lazy(() =>
  Promise.all([
    import("react-markdown"),
    import("remark-gfm"),
    import("rehype-highlight"),
    import("highlight.js/lib/common"),
    import("mermaid"),
  ]).then(([md, gfm, rehypeHL, _hljs, mermaidMod]) => {
    const ReactMarkdown = md.default
    const remarkGfm = gfm.default
    const rehypeHighlight = rehypeHL.default
    const mermaid = mermaidMod.default
    mermaid.initialize({ startOnLoad: false, theme: "default" })

    let mermaidCounter = 0

    function MermaidBlock({ children }: { children: string }) {
      const containerRef = React.useRef<HTMLDivElement>(null)
      const idRef = React.useRef(`mermaid-${++mermaidCounter}`)

      React.useEffect(() => {
        const el = containerRef.current
        if (!el) return
        const id = idRef.current
        const isDark = document.documentElement.classList.contains("dark")
        mermaid.initialize({ startOnLoad: false, theme: isDark ? "dark" : "default" })
        mermaid
          .render(id, children.trim())
          .then(({ svg }) => {
            const parser = new DOMParser()
            const doc = parser.parseFromString(svg, "image/svg+xml")
            const svgEl = doc.documentElement
            el.replaceChildren(svgEl)
          })
          .catch(() => {
            el.textContent = children
          })
      }, [children])

      return <div ref={containerRef} className="my-2 flex justify-center" />
    }

    function LazyMarkdownInner({ children }: { children: string }) {
      return (
        <ReactMarkdown
          remarkPlugins={[remarkGfm]}
          rehypePlugins={[rehypeHighlight]}
          components={{
            code({ className, children: codeChildren, ...props }) {
              const match = /language-mermaid/.test(className || "")
              const text = String(codeChildren).replace(/\n$/, "")
              if (match) {
                return <MermaidBlock>{text}</MermaidBlock>
              }
              return (
                <code className={className} {...props}>
                  {codeChildren}
                </code>
              )
            },
            pre({ children: preChildren }) {
              const child = Array.isArray(preChildren) ? preChildren[0] : preChildren
              const childEl = child as
                | React.ReactElement<{
                    className?: string
                  }>
                | undefined
              if (childEl?.props?.className?.includes("language-mermaid")) {
                return <>{preChildren}</>
              }
              return <pre>{preChildren}</pre>
            },
          }}
        >
          {children}
        </ReactMarkdown>
      )
    }
    LazyMarkdownInner.displayName = "LazyMarkdown"
    return { default: LazyMarkdownInner }
  }),
)

function HighlightedText({ text, search }: { text: string; search: string }) {
  if (!search) return <>{text}</>
  const parts: React.ReactNode[] = []
  const lower = text.toLowerCase()
  const needle = search.toLowerCase()
  let last = 0
  let idx = lower.indexOf(needle, last)
  while (idx !== -1) {
    if (idx > last) parts.push(text.slice(last, idx))
    parts.push(
      <mark key={idx} className="rounded-sm bg-yellow-300/80 px-0.5 dark:bg-yellow-500/40">
        {text.slice(idx, idx + needle.length)}
      </mark>,
    )
    last = idx + needle.length
    idx = lower.indexOf(needle, last)
  }
  if (last < text.length) parts.push(text.slice(last))
  return <>{parts}</>
}

export function CodeBlock({ label, content }: { label: string; content: string }) {
  const { t } = useTranslation("logs")
  const [copied, setCopied] = useState(false)
  const [searchTerm, setSearchTerm] = useState("")
  const { resolvedTheme } = useTheme()

  const displayContent = content?.trim() || ""

  const parsed = useMemo(() => {
    if (!displayContent) return { isJson: false, data: null, truncated: false }
    try {
      return { isJson: true, data: JSON.parse(displayContent), truncated: false }
    } catch {
      const repaired = repairTruncatedJson(displayContent)
      if (repaired) return { isJson: true, ...repaired }
      return { isJson: false, data: displayContent, truncated: false }
    }
  }, [displayContent])

  const plainText = useMemo(() => {
    if (!displayContent) return ""
    return parsed.isJson ? JSON.stringify(parsed.data, null, 2) : displayContent
  }, [displayContent, parsed])

  const matchCount = useMemo(() => {
    return countMatches(plainText, searchTerm)
  }, [plainText, searchTerm])

  if (!displayContent) {
    return (
      <div className="flex flex-col gap-2">
        <p className="text-muted-foreground text-sm">{t("codeBlock.noContent")}</p>
      </div>
    )
  }

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(plainText)
      setCopied(true)
      toast.success(t("toast.copied"))
      setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error(t("actions.copyFailed", { ns: "common" }))
    }
  }

  return (
    <div className="flex min-w-0 flex-col gap-2">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-xs font-medium">{label}</p>
        <Button variant="ghost" size="sm" className="h-7 gap-1" onClick={handleCopy}>
          {copied ? <Check className="h-3 w-3" /> : <Copy className="h-3 w-3" />}
          <span className="text-xs">
            {copied ? t("actions.copied", { ns: "common" }) : t("actions.copy", { ns: "common" })}
          </span>
        </Button>
      </div>
      <div className="relative">
        <Search className="text-muted-foreground absolute top-2 left-2.5 h-3.5 w-3.5" />
        <Input
          placeholder={t("codeBlock.searchPlaceholder")}
          aria-label={t("codeBlock.searchPlaceholder")}
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
          className="h-8 pr-16 pl-8 text-xs"
        />
        {searchTerm && (
          <div className="absolute top-1.5 right-2 flex items-center gap-1">
            <span className="text-muted-foreground text-xs">
              {t("codeBlock.match", { count: matchCount })}
            </span>
            <button onClick={() => setSearchTerm("")}>
              <X className="text-muted-foreground h-3 w-3" />
            </button>
          </div>
        )}
      </div>
      {searchTerm ? (
        <div className="bg-muted/30 max-h-[50vh] min-w-0 overflow-auto rounded-xl border p-4 shadow-sm">
          <pre
            className="text-xs break-words whitespace-pre-wrap"
            style={{
              fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace",
            }}
          >
            <HighlightedText text={plainText} search={searchTerm} />
          </pre>
        </div>
      ) : parsed.isJson ? (
        <div className="flex flex-col gap-2">
          {parsed.truncated && (
            <p className="text-muted-foreground text-xs italic">
              {t("codeBlock.truncatedRecovered")}
            </p>
          )}
          <div className="bg-muted/30 max-h-[50vh] min-w-0 overflow-auto rounded-xl border p-4 shadow-sm">
            <Suspense
              fallback={
                <div className="flex flex-col gap-2 p-3">
                  <Skeleton className="h-4 w-3/4" />
                  <Skeleton className="h-4 w-1/2" />
                  <Skeleton className="h-4 w-2/3" />
                </div>
              }
            >
              <LazyJsonView
                isDark={resolvedTheme === "dark"}
                value={parsed.data}
                style={{
                  fontSize: "12px",
                  backgroundColor: "transparent",
                  fontFamily: "ui-monospace, SFMono-Regular, 'SF Mono', Menlo, Consolas, monospace",
                }}
                displayDataTypes={false}
                displayObjectSize={false}
                collapsed={2}
              />
            </Suspense>
          </div>
        </div>
      ) : (
        <div className="bg-muted/30 prose prose-sm dark:prose-invert max-h-[50vh] max-w-none overflow-auto rounded-xl border p-4 break-words shadow-sm">
          <Suspense
            fallback={
              <div className="flex flex-col gap-2 p-3">
                <Skeleton className="h-4 w-full" />
                <Skeleton className="h-4 w-4/5" />
              </div>
            }
          >
            <LazyMarkdown>{displayContent}</LazyMarkdown>
          </Suspense>
        </div>
      )}
    </div>
  )
}

export function CollapsibleCodeBlock({
  label,
  content,
  defaultOpen = true,
}: {
  label: string
  content: string
  defaultOpen?: boolean
}) {
  const [open, setOpen] = useState(defaultOpen)

  return (
    <div className="overflow-hidden rounded-xl border shadow-sm">
      <button
        type="button"
        className="bg-muted/50 hover:bg-muted/80 flex w-full items-center justify-between px-4 py-3 transition-colors"
        onClick={() => setOpen(!open)}
      >
        <span className="text-xs font-medium">{label}</span>
        <ChevronDown
          className={`text-muted-foreground h-4 w-4 transition-transform ${open ? "" : "-rotate-90"}`}
        />
      </button>
      {open && (
        <div className="p-4 pt-2">
          <CodeBlock label="" content={content} />
        </div>
      )}
    </div>
  )
}

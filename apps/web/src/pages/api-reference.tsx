import { BookOpen, ExternalLink, Loader2 } from "lucide-react"
import { useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { getApiBaseUrl } from "@/lib/api-client"

type DocsStatus = "loading" | "available" | "unavailable"

export default function ApiReferencePage() {
  const { t } = useTranslation("api-reference")
  const baseUrl = getApiBaseUrl() || window.location.origin
  const docsUrl = `${baseUrl}/docs`

  const [status, setStatus] = useState<DocsStatus>("loading")

  // Probe whether the API docs endpoint actually exists before embedding it.
  // The SPA fallback always returns 200 with its own index.html for unknown
  // paths, so a HEAD/status check is not enough. We fetch the content and
  // verify it looks like the Scalar API Reference page rather than the SPA shell.
  useEffect(() => {
    let cancelled = false
    fetch(docsUrl)
      .then(async (res) => {
        if (cancelled) return
        if (!res.ok) {
          setStatus("unavailable")
          return
        }
        const contentType = res.headers.get("content-type") ?? ""
        if (!contentType.includes("text/html")) {
          setStatus("unavailable")
          return
        }
        // Read a small prefix to check for API docs markers
        const text = await res.text()
        const prefix = text.slice(0, 4096).toLowerCase()
        const isDocs =
          prefix.includes("scalar") ||
          prefix.includes("api-reference") ||
          prefix.includes("openapi") ||
          prefix.includes("swagger")
        setStatus(isDocs ? "available" : "unavailable")
      })
      .catch(() => {
        if (!cancelled) setStatus("unavailable")
      })
    return () => {
      cancelled = true
    }
  }, [docsUrl])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between pb-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
          <p className="text-muted-foreground text-sm">{t("description")}</p>
        </div>
        <Button variant="outline" size="sm" asChild>
          <a href={docsUrl} target="_blank" rel="noopener noreferrer">
            {t("openInNewTab")}
            <ExternalLink className="ml-2 h-4 w-4" />
          </a>
        </Button>
      </div>

      <div className="min-h-0 flex-1 overflow-hidden rounded-lg border">
        {status === "loading" && (
          <div className="flex h-full flex-col items-center justify-center gap-3">
            <Loader2 className="text-muted-foreground h-8 w-8 animate-spin" />
            <p className="text-muted-foreground text-sm">{t("loading")}</p>
          </div>
        )}
        {status === "unavailable" && (
          <div className="flex h-full flex-col items-center justify-center gap-4 p-8">
            <BookOpen className="text-muted-foreground h-12 w-12" />
            <div className="text-center">
              <p className="text-foreground font-medium">{t("unavailable")}</p>
              <p className="text-muted-foreground mt-1 text-sm">{t("unavailableHint")}</p>
            </div>
            <div className="bg-muted mt-2 rounded-lg px-4 py-2 font-mono text-sm select-all">
              {docsUrl}
            </div>
          </div>
        )}
        {status === "available" && (
          <iframe
            src={docsUrl}
            title={t("title")}
            className="h-full w-full border-0"
            sandbox="allow-scripts allow-same-origin allow-forms allow-popups"
          />
        )}
      </div>
    </div>
  )
}

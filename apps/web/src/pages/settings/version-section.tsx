import { ExternalLink, RefreshCw } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import Markdown from "react-markdown"
import remarkGfm from "remark-gfm"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { checkUpdate } from "@/lib/api-client"

interface UpdateInfo {
  current: string
  latest: string
  hasUpdate: boolean
  releaseUrl: string
  releaseNotes: string
}

export default function VersionSection() {
  const { t } = useTranslation("settings")
  const [checking, setChecking] = useState(false)
  const [update, setUpdate] = useState<UpdateInfo | null>(null)

  const handleCheckUpdate = async () => {
    setChecking(true)
    setUpdate(null)
    try {
      const res = await checkUpdate()
      if (res.success) {
        setUpdate(res.data)
        if (!res.data.hasUpdate) {
          toast.success(t("version.upToDate"))
        }
      } else {
        toast.error(t("version.checkFailed"))
      }
    } catch {
      toast.error(t("version.checkFailed"))
    } finally {
      setChecking(false)
    }
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("version.title")}</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        <div className="flex items-center gap-3">
          <span className="text-muted-foreground text-sm">{t("version.currentVersion")}</span>
          <Badge variant="secondary">v{__APP_VERSION__}</Badge>
        </div>

        <div className="flex items-center gap-2">
          <Button variant="outline" size="sm" onClick={handleCheckUpdate} disabled={checking}>
            <RefreshCw className={`mr-2 h-4 w-4 ${checking ? "animate-spin" : ""}`} />
            {checking ? t("version.checking") : t("version.checkUpdate")}
          </Button>
        </div>

        {update?.hasUpdate && (
          <div className="flex flex-col gap-3 rounded-md border p-4">
            <div className="flex items-center justify-between">
              <span className="text-sm font-medium">
                {t("version.newVersion", { version: update.latest })}
              </span>
              <Button variant="outline" size="sm" asChild>
                <a href={update.releaseUrl} target="_blank" rel="noopener noreferrer">
                  <ExternalLink className="mr-2 h-4 w-4" />
                  {t("version.goToDownload")}
                </a>
              </Button>
            </div>

            {update.releaseNotes && (
              <div>
                <p className="text-muted-foreground mb-2 text-xs font-medium">
                  {t("version.releaseNotes")}
                </p>
                <div className="prose prose-sm dark:prose-invert bg-muted/50 max-h-64 max-w-none overflow-y-auto rounded-md p-3">
                  <Markdown remarkPlugins={[remarkGfm]}>{update.releaseNotes}</Markdown>
                </div>
              </div>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}

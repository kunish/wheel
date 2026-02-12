import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { useAuthStore } from "@/lib/store/auth"

export default function ConnectionSection() {
  const { t } = useTranslation("settings")
  const apiBaseUrl = useAuthStore((s) => s.apiBaseUrl)
  const setApiBaseUrl = useAuthStore((s) => s.setApiBaseUrl)
  const [url, setUrl] = useState(apiBaseUrl)
  const [testing, setTesting] = useState(false)

  async function handleSave() {
    const trimmed = url.replace(/\/+$/, "")
    setTesting(true)
    try {
      const target = trimmed ? `${trimmed}/` : "/"
      const resp = await fetch(target, { method: "GET" })
      if (!resp.ok) throw new Error("Connection failed")
      const data = await resp.json()
      if (data.name !== "wheel") throw new Error("Not a Wheel server")
      setApiBaseUrl(trimmed)
      toast.success(t("connection.success"))
    } catch {
      toast.error(t("connection.failed"))
    } finally {
      setTesting(false)
    }
  }

  function handleClear() {
    setUrl("")
    setApiBaseUrl("")
    toast.success(t("connection.cleared"))
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("connection.title")}</CardTitle>
      </CardHeader>
      <CardContent>
        <div className="flex flex-col gap-4">
          <div className="flex flex-col gap-2">
            <Label>{t("connection.apiUrl")}</Label>
            <p className="text-muted-foreground text-xs">{t("connection.apiUrlHint")}</p>
            <div className="flex gap-2">
              <Input
                type="url"
                value={url}
                onChange={(e) => setUrl(e.target.value)}
                placeholder={t("connection.apiUrlPlaceholder")}
              />
              <Button onClick={handleSave} disabled={testing}>
                {testing ? t("connection.testing") : t("connection.save")}
              </Button>
              {apiBaseUrl && (
                <Button variant="outline" onClick={handleClear}>
                  &times;
                </Button>
              )}
            </div>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

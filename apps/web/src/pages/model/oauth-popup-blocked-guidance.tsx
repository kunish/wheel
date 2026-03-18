import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"

interface OAuthPopupBlockedGuidanceProps {
  warningCode?: string
  warningMessage?: string
  canRetry: boolean
  onRetry: () => void
}

export function OAuthPopupBlockedGuidance({
  warningCode,
  warningMessage,
  canRetry,
  onRetry,
}: OAuthPopupBlockedGuidanceProps) {
  const { t } = useTranslation("model")

  if (warningCode !== "popup_blocked" || !warningMessage) {
    return null
  }

  return (
    <Card className="border-amber-300 bg-amber-50">
      <CardHeader>
        <CardTitle>{t("runtime.oauth.dialog.popupBlockedTitle")}</CardTitle>
        <CardDescription>{warningMessage}</CardDescription>
      </CardHeader>
      {canRetry ? (
        <CardContent>
          <Button type="button" variant="outline" size="sm" onClick={onRetry}>
            {t("runtime.oauth.dialog.retry")}
          </Button>
        </CardContent>
      ) : null}
    </Card>
  )
}

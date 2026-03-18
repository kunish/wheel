import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { OAuthPopupBlockedGuidance } from "./oauth-popup-blocked-guidance"

export interface OAuthDeviceCodeFlowProps {
  oauthUrl: string
  userCode: string
  verificationUri: string
  warningCode?: string
  warningMessage?: string
  canRetry: boolean
  onOpenAuthPage: () => boolean | void
  onCopyAuthLink: (value: string) => void
  onCopyUserCode: () => void
  onRetry: () => void
}

export function OAuthDeviceCodeFlow({
  oauthUrl,
  userCode,
  verificationUri,
  warningCode,
  warningMessage,
  canRetry,
  onOpenAuthPage,
  onCopyAuthLink,
  onCopyUserCode,
  onRetry,
}: OAuthDeviceCodeFlowProps) {
  const { t } = useTranslation("model")
  const effectiveVerificationUrl = verificationUri || oauthUrl

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("runtime.oauth.dialog.deviceCodeTitle")}</CardTitle>
          <CardDescription>{t("runtime.oauth.dialog.deviceCodeDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="bg-muted rounded-md border px-3 py-2 font-mono text-lg font-semibold tracking-[0.2em]">
            {userCode}
          </div>
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" onClick={onCopyUserCode}>
              {t("codex.oauthCopyCode")}
            </Button>
            <Button type="button" variant="outline" size="sm" onClick={() => onOpenAuthPage()}>
              {t("codex.oauthOpenLink")}
            </Button>
          </div>
          <a
            href={effectiveVerificationUrl}
            target="_blank"
            rel="noreferrer"
            className="text-primary text-sm break-all underline-offset-4 hover:underline"
          >
            {effectiveVerificationUrl}
          </a>
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => onCopyAuthLink(effectiveVerificationUrl)}
          >
            {t("codex.oauthCopyLink")}
          </Button>
        </CardContent>
      </Card>

      <OAuthPopupBlockedGuidance
        warningCode={warningCode}
        warningMessage={warningMessage}
        canRetry={canRetry}
        onRetry={onRetry}
      />
    </div>
  )
}

import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { OAuthPopupBlockedGuidance } from "./oauth-popup-blocked-guidance"

export interface OAuthCallbackValidation {
  code: "empty" | "ok" | string
  message?: string
}

export interface OAuthRedirectFlowProps {
  oauthUrl: string
  callbackInput: string
  callbackValidation: OAuthCallbackValidation
  warningCode?: string
  warningMessage?: string
  isSubmittingCallback: boolean
  canRetry: boolean
  onOpenAuthPage: () => boolean | void
  onCopyAuthLink: (value: string) => void
  onPasteCallback: () => void
  onCallbackInputChange: (value: string) => void
  onSubmitCallback: () => void
  onRetry: () => void
}

export function OAuthRedirectFlow({
  oauthUrl,
  callbackInput,
  callbackValidation,
  warningCode,
  warningMessage,
  isSubmittingCallback,
  canRetry,
  onOpenAuthPage,
  onCopyAuthLink,
  onPasteCallback,
  onCallbackInputChange,
  onSubmitCallback,
  onRetry,
}: OAuthRedirectFlowProps) {
  const { t } = useTranslation("model")
  const hasValidationError = callbackValidation.code !== "empty" && callbackValidation.code !== "ok"

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>{t("runtime.oauth.dialog.redirectTitle")}</CardTitle>
          <CardDescription>{t("runtime.oauth.dialog.redirectDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <a
            href={oauthUrl}
            target="_blank"
            rel="noreferrer"
            className="text-primary text-sm break-all underline-offset-4 hover:underline"
          >
            {oauthUrl}
          </a>
          <div className="flex flex-wrap gap-2">
            <Button type="button" size="sm" onClick={() => onOpenAuthPage()}>
              {t("codex.oauthOpenLink")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => onCopyAuthLink(oauthUrl)}
            >
              {t("codex.oauthCopyLink")}
            </Button>
          </div>
          <p className="text-muted-foreground text-xs">{t("runtime.oauth.dialog.redirectHelp")}</p>
        </CardContent>
      </Card>

      <OAuthPopupBlockedGuidance
        warningCode={warningCode}
        warningMessage={warningMessage}
        canRetry={canRetry}
        onRetry={onRetry}
      />

      <Card>
        <CardHeader>
          <CardTitle>{t("runtime.oauth.dialog.callbackTitle")}</CardTitle>
          <CardDescription>{t("runtime.oauth.dialog.callbackDescription")}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="flex flex-col gap-2 sm:flex-row">
            <Input
              value={callbackInput}
              onChange={(event) => onCallbackInputChange(event.target.value)}
              placeholder={t("codex.oauthCallbackPlaceholder")}
              aria-invalid={hasValidationError}
            />
            <Button type="button" variant="outline" size="sm" onClick={onPasteCallback}>
              {t("runtime.oauth.dialog.pasteFromClipboard")}
            </Button>
          </div>
          {hasValidationError && callbackValidation.message ? (
            <p className="text-destructive text-sm">{callbackValidation.message}</p>
          ) : null}
          <Button
            type="button"
            size="sm"
            disabled={!callbackInput.trim() || isSubmittingCallback}
            onClick={onSubmitCallback}
          >
            {isSubmittingCallback
              ? t("runtime.oauth.dialog.importing")
              : t("codex.oauthCallbackSubmit")}
          </Button>
        </CardContent>
      </Card>
    </div>
  )
}

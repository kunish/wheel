import type { OAuthCallbackValidation } from "./oauth-redirect-flow"
import type { RuntimeOAuthFlowType } from "@/lib/api/codex"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { OAuthDeviceCodeFlow } from "./oauth-device-code-flow"
import { OAuthRedirectFlow } from "./oauth-redirect-flow"

export interface OAuthFlowDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  flowType: RuntimeOAuthFlowType
  oauthUrl: string
  userCode: string
  verificationUri: string
  callbackInput: string
  callbackValidation: OAuthCallbackValidation
  warningCode?: string
  warningMessage?: string
  canRetry: boolean
  isSubmittingCallback: boolean
  onOpenAuthPage: () => boolean | void
  onCopyAuthLink: (value: string) => void
  onCopyUserCode: () => void
  onPasteCallback: () => void
  onCallbackInputChange: (value: string) => void
  onSubmitCallback: () => void
  onRetry: () => void
}

export function OAuthFlowDialog({
  open,
  onOpenChange,
  title,
  description,
  flowType,
  oauthUrl,
  userCode,
  verificationUri,
  callbackInput,
  callbackValidation,
  warningCode,
  warningMessage,
  canRetry,
  isSubmittingCallback,
  onOpenAuthPage,
  onCopyAuthLink,
  onCopyUserCode,
  onPasteCallback,
  onCallbackInputChange,
  onSubmitCallback,
  onRetry,
}: OAuthFlowDialogProps) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-w-lg">
        <DialogHeader>
          <DialogTitle>{title}</DialogTitle>
          {description ? <DialogDescription>{description}</DialogDescription> : null}
        </DialogHeader>

        {flowType === "device_code" ? (
          <OAuthDeviceCodeFlow
            oauthUrl={oauthUrl}
            userCode={userCode}
            verificationUri={verificationUri}
            warningCode={warningCode}
            warningMessage={warningMessage}
            canRetry={canRetry}
            onOpenAuthPage={onOpenAuthPage}
            onCopyAuthLink={onCopyAuthLink}
            onCopyUserCode={onCopyUserCode}
            onRetry={onRetry}
          />
        ) : (
          <OAuthRedirectFlow
            oauthUrl={oauthUrl}
            callbackInput={callbackInput}
            callbackValidation={callbackValidation}
            warningCode={warningCode}
            warningMessage={warningMessage}
            isSubmittingCallback={isSubmittingCallback}
            canRetry={canRetry}
            onOpenAuthPage={onOpenAuthPage}
            onCopyAuthLink={onCopyAuthLink}
            onPasteCallback={onPasteCallback}
            onCallbackInputChange={onCallbackInputChange}
            onSubmitCallback={onSubmitCallback}
            onRetry={onRetry}
          />
        )}
      </DialogContent>
    </Dialog>
  )
}

import type { CodexAuthFile } from "@/lib/api"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Check,
  Copy,
  ExternalLink,
  Eye,
  Loader2,
  LogIn,
  RefreshCw,
  Trash2,
  Upload,
  XCircle,
} from "lucide-react"
import { useCallback, useEffect, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Progress } from "@/components/ui/progress"
import { Switch } from "@/components/ui/switch"
import {
  deleteCodexAuthFile,
  getCodexAuthFileModels,
  getCodexAuthUploadToastState,
  getCodexOAuthStatus,
  listCodexAuthFiles,
  listCodexQuota,
  patchCodexAuthFileStatus,
  startCodexOAuth,
  syncCodexKeys,
  uploadCodexAuthFile,
} from "@/lib/api"
import {
  channelsQueryKey,
  codexAuthFilesQueryKey,
  codexQuotaQueryKey,
  codexUploadRefreshQueryKeys,
} from "./codex-query-keys"

interface CodexChannelDetailProps {
  channelId: number
}

export function CodexChannelDetail({ channelId }: CodexChannelDetailProps) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const [modelsDialogFile, setModelsDialogFile] = useState<CodexAuthFile | null>(null)

  const authQuery = useQuery({
    queryKey: codexAuthFilesQueryKey(channelId),
    queryFn: () => listCodexAuthFiles(channelId, { provider: "codex" }),
  })

  const quotaQuery = useQuery({
    queryKey: codexQuotaQueryKey(channelId),
    queryFn: () => listCodexQuota(channelId),
  })

  const modelsQuery = useQuery({
    queryKey: ["codex-auth-models", channelId, modelsDialogFile?.name],
    queryFn: () => getCodexAuthFileModels(channelId, modelsDialogFile?.name || ""),
    enabled: !!modelsDialogFile,
  })

  const toggleMut = useMutation({
    mutationFn: (input: { name: string; disabled: boolean }) =>
      patchCodexAuthFileStatus(channelId, input),
    onSuccess: () => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.statusUpdated"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (name: string) => deleteCodexAuthFile(channelId, { name }),
    onSuccess: () => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.authDeleted"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const syncMut = useMutation({
    mutationFn: () => syncCodexKeys(channelId),
    onSuccess: (res) => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.syncSuccess", { count: res.data.synced }))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const uploadMut = useMutation({
    mutationFn: (files: File[]) => uploadCodexAuthFile(channelId, files),
    onSuccess: (res) => {
      if (res.data.successCount > 0) {
        for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
          void queryClient.invalidateQueries({ queryKey })
        }
      }

      const toastState = getCodexAuthUploadToastState(res.data)
      toast[toastState.level](t(toastState.key, toastState.values))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const [oauthPanelOpen, setOauthPanelOpen] = useState(false)
  const [oauthUrl, setOauthUrl] = useState("")
  const [_oauthState, setOauthState] = useState("")
  const [oauthStatus, setOauthStatus] = useState<
    "idle" | "starting" | "waiting" | "success" | "error"
  >("idle")
  const [oauthError, setOauthError] = useState("")
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)
  const uploadInputRef = useRef<HTMLInputElement | null>(null)

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  // Cleanup polling on unmount
  useEffect(() => {
    return () => stopPolling()
  }, [stopPolling])

  const handleStartOAuth = useCallback(async () => {
    setOauthStatus("starting")
    setOauthError("")
    setOauthUrl("")
    setOauthState("")
    try {
      const res = await startCodexOAuth(channelId)
      const { url, state } = res.data
      setOauthUrl(url)
      setOauthState(state)
      setOauthStatus("waiting")

      // Start polling for status
      const startTime = Date.now()
      pollRef.current = setInterval(async () => {
        // Timeout after 5 minutes
        if (Date.now() - startTime > 5 * 60 * 1000) {
          stopPolling()
          setOauthStatus("error")
          setOauthError(t("codex.oauthTimeout"))
          return
        }
        try {
          const statusRes = await getCodexOAuthStatus(channelId, state)
          const { status, error } = statusRes.data
          if (status === "ok") {
            stopPolling()
            setOauthStatus("success")
            for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
              void queryClient.invalidateQueries({ queryKey })
            }
          } else if (status === "error") {
            stopPolling()
            setOauthStatus("error")
            setOauthError(error || "Unknown error")
          }
          // status === "wait" → keep polling
        } catch {
          // Network error during poll — keep trying
        }
      }, 3000)
    } catch (err) {
      setOauthStatus("error")
      setOauthError(err instanceof Error ? err.message : "Failed to start OAuth")
    }
  }, [channelId, queryClient, stopPolling, t])

  const handleOauthDialogChange = useCallback(
    (open: boolean) => {
      if (!open) {
        stopPolling()
        setOauthStatus("idle")
        setOauthUrl("")
        setOauthState("")
        setOauthError("")
      }
      setOauthPanelOpen(open)
    },
    [stopPolling],
  )

  const authFiles = authQuery.data?.data.files ?? []
  const quotaItems = quotaQuery.data?.data.items ?? []
  const capabilities = authQuery.data?.data.capabilities

  const handleUploadFile = useCallback(
    (fileList: FileList | null | undefined) => {
      const files = Array.from(fileList ?? [])
      if (files.length === 0) {
        return
      }
      if (files.some((file) => !file.name.toLowerCase().endsWith(".json"))) {
        toast.error(t("codex.invalidJsonFile"))
        return
      }

      uploadMut.mutate(files)
    },
    [t, uploadMut],
  )

  const remainingPercent = (usedPercent?: number) =>
    Math.max(0, 100 - Math.min(usedPercent || 0, 100))

  return (
    <div className="mt-2 space-y-3">
      {/* Auth Files Section */}
      <div className="space-y-2">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <h5 className="text-muted-foreground shrink-0 text-xs font-medium tracking-wide whitespace-nowrap uppercase">
            {t("codex.authFiles")}
          </h5>
          <div className="flex flex-wrap items-center gap-1 lg:justify-end">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => {
                void queryClient.invalidateQueries({ queryKey: codexAuthFilesQueryKey(channelId) })
                void queryClient.invalidateQueries({ queryKey: codexQuotaQueryKey(channelId) })
                void queryClient.invalidateQueries({ queryKey: channelsQueryKey })
              }}
            >
              <RefreshCw className="mr-1 h-3 w-3" />
              {t("codex.refresh")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => syncMut.mutate()}
              disabled={syncMut.isPending}
            >
              {syncMut.isPending && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
              {t("codex.syncKeys")}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={() => uploadInputRef.current?.click()}
              disabled={uploadMut.isPending}
            >
              {uploadMut.isPending ? (
                <Loader2 className="mr-1 h-3 w-3 animate-spin" />
              ) : (
                <Upload className="mr-1 h-3 w-3" />
              )}
              {uploadMut.isPending ? t("codex.uploadingFile") : t("codex.importFile")}
            </Button>
            <input
              ref={uploadInputRef}
              type="file"
              multiple
              accept=".json"
              className="hidden"
              onChange={(e) => {
                handleUploadFile(e.target.files)
                e.currentTarget.value = ""
              }}
            />
            {capabilities?.oauthEnabled !== false && (
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="h-7 text-xs"
                onClick={() => {
                  setOauthPanelOpen(true)
                  void handleStartOAuth()
                }}
              >
                <LogIn className="mr-1 h-3 w-3" />
                {t("codex.importOAuth")}
              </Button>
            )}
          </div>
        </div>

        {authQuery.isLoading ? (
          <p className="text-muted-foreground text-xs">{t("actions.loading", { ns: "common" })}</p>
        ) : authFiles.length === 0 ? (
          <div className="text-muted-foreground rounded-md border px-3 py-2 text-xs">
            {t("codex.noAuthFiles")}
          </div>
        ) : (
          <div className="space-y-1.5">
            {authFiles.map((file) => {
              const disabled = !!file.disabled
              return (
                <div
                  key={file.name}
                  className="flex flex-col gap-2 rounded-md border px-2.5 py-1.5 text-sm lg:flex-row lg:items-center lg:justify-between"
                >
                  <div className="min-w-0 flex-1">
                    <div className="flex items-center gap-1.5">
                      <Badge variant="secondary" className="px-1.5 py-0 text-[10px]">
                        {file.provider || "codex"}
                      </Badge>
                      <span className="truncate text-xs font-medium">{file.name}</span>
                    </div>
                    <p className="text-muted-foreground mt-0.5 text-[10px]">
                      {file.email || t("codex.noEmail")}
                    </p>
                  </div>
                  <div className="flex shrink-0 items-center justify-end gap-1 self-end lg:self-auto">
                    {capabilities?.modelsEnabled !== false && (
                      <Button
                        type="button"
                        size="icon"
                        variant="ghost"
                        className="h-6 w-6"
                        onClick={() => setModelsDialogFile(file)}
                      >
                        <Eye className="h-3 w-3" />
                      </Button>
                    )}
                    <Button
                      type="button"
                      size="icon"
                      variant="ghost"
                      className="h-6 w-6"
                      onClick={() => deleteMut.mutate(file.name)}
                      disabled={deleteMut.isPending}
                    >
                      <Trash2 className="h-3 w-3" />
                    </Button>
                    <Switch
                      checked={!disabled}
                      onCheckedChange={(checked) =>
                        toggleMut.mutate({ name: file.name, disabled: !checked })
                      }
                      disabled={toggleMut.isPending}
                      className="scale-75"
                    />
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </div>

      {/* Quota Section */}
      {quotaItems.length > 0 && (
        <div className="space-y-2">
          <h5 className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {t("codex.remainingQuota")}
          </h5>
          <div className="grid gap-1.5 lg:grid-cols-2">
            {quotaItems.map((item) => (
              <div key={item.name} className="rounded-md border px-2.5 py-2 text-xs">
                <div className="mb-1.5 flex items-center justify-between gap-1">
                  <span className="truncate font-medium">{item.name}</span>
                  {item.planType && (
                    <Badge variant="outline" className="px-1 py-0 text-[10px]">
                      {item.planType}
                    </Badge>
                  )}
                </div>
                {item.error ? (
                  <p className="text-destructive text-[10px]">{item.error}</p>
                ) : (
                  <div className="space-y-1.5">
                    <div>
                      <div className="mb-0.5 flex items-center justify-between text-[10px]">
                        <span>{t("codex.weekly")}</span>
                        <span>{remainingPercent(item.weekly.usedPercent).toFixed(1)}%</span>
                      </div>
                      <Progress
                        value={remainingPercent(item.weekly.usedPercent)}
                        className="h-1.5"
                      />
                    </div>
                    <div>
                      <div className="mb-0.5 flex items-center justify-between text-[10px]">
                        <span>{t("codex.codeReview")}</span>
                        <span>{remainingPercent(item.codeReview.usedPercent).toFixed(1)}%</span>
                      </div>
                      <Progress
                        value={remainingPercent(item.codeReview.usedPercent)}
                        className="h-1.5"
                      />
                    </div>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* Models Dialog */}
      <Dialog open={!!modelsDialogFile} onOpenChange={(open) => !open && setModelsDialogFile(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>
              {t("codex.modelsFor", { name: modelsDialogFile?.name || "" })}
            </DialogTitle>
          </DialogHeader>
          {modelsQuery.isLoading ? (
            <div className="text-muted-foreground flex items-center gap-2 text-sm">
              <Loader2 className="h-4 w-4 animate-spin" />
              {t("actions.loading", { ns: "common" })}
            </div>
          ) : (
            <div className="max-h-96 space-y-2 overflow-auto">
              {(modelsQuery.data?.data.models ?? []).map((model, idx) => (
                <div key={String(model.id || idx)} className="rounded-md border p-2 text-sm">
                  <div className="font-medium">{String(model.id || "-")}</div>
                  {model.display_name ? (
                    <div className="text-muted-foreground text-xs">
                      {String(model.display_name)}
                    </div>
                  ) : null}
                </div>
              ))}
            </div>
          )}
        </DialogContent>
      </Dialog>

      {/* OAuth Flow Dialog */}
      <Dialog open={oauthPanelOpen} onOpenChange={handleOauthDialogChange}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{t("codex.importOAuth")}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <p className="text-muted-foreground text-xs">{t("codex.oauthHint")}</p>

            {oauthStatus === "starting" && (
              <div className="flex items-center gap-2 text-sm">
                <Loader2 className="h-4 w-4 animate-spin" />
                <span>{t("codex.oauthStarting")}</span>
              </div>
            )}

            {oauthStatus === "waiting" && oauthUrl && (
              <div className="space-y-3">
                <div className="rounded-md border p-3">
                  <p className="mb-2 text-xs font-medium break-all">{oauthUrl}</p>
                  <div className="flex gap-2">
                    <Button
                      type="button"
                      variant="outline"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => {
                        void navigator.clipboard.writeText(oauthUrl)
                        toast.success(t("codex.oauthLinkCopied"))
                      }}
                    >
                      <Copy className="mr-1 h-3 w-3" />
                      {t("codex.oauthCopyLink")}
                    </Button>
                    <Button
                      type="button"
                      variant="default"
                      size="sm"
                      className="h-7 text-xs"
                      onClick={() => window.open(oauthUrl, "_blank")}
                    >
                      <ExternalLink className="mr-1 h-3 w-3" />
                      {t("codex.oauthOpenLink")}
                    </Button>
                  </div>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <Loader2 className="h-4 w-4 animate-spin" />
                  <span>{t("codex.oauthWaiting")}</span>
                </div>
              </div>
            )}

            {oauthStatus === "success" && (
              <div className="flex items-center gap-2 text-sm text-green-600">
                <Check className="h-4 w-4" />
                <span>{t("codex.oauthSuccess")}</span>
              </div>
            )}

            {oauthStatus === "error" && (
              <div className="flex items-center gap-2 text-sm text-red-600">
                <XCircle className="h-4 w-4" />
                <span>{t("codex.oauthError", { error: oauthError })}</span>
              </div>
            )}
          </div>
        </DialogContent>
      </Dialog>
    </div>
  )
}

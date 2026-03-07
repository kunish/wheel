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
  runtimeProviderFilter,
  startCodexOAuth,
  syncCodexKeys,
  uploadCodexAuthFile,
} from "@/lib/api"
import { getRuntimeProviderKey } from "./codex-channel-draft"
import {
  channelsQueryKey,
  codexAuthFilesQueryKey,
  codexQuotaQueryKey,
  codexUploadRefreshQueryKeys,
} from "./codex-query-keys"

const AUTH_FILES_PAGE_SIZE = 8
const QUOTA_PAGE_SIZE = 6

interface CodexChannelDetailProps {
  channelId: number
  channelType?: number
  modelCount?: number
}

export function CodexChannelDetail({
  channelId,
  channelType,
  modelCount = 0,
}: CodexChannelDetailProps) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const [modelsDialogFile, setModelsDialogFile] = useState<CodexAuthFile | null>(null)
  const [authPage, setAuthPage] = useState(1)
  const [quotaPage, setQuotaPage] = useState(1)
  const providerKey = getRuntimeProviderKey(channelType)
  const providerLabel = providerKey ? t(`typeLabels.${channelType}`) : t("typeLabels.33")

  const authQuery = useQuery({
    queryKey: codexAuthFilesQueryKey(channelId, {
      page: authPage,
      pageSize: AUTH_FILES_PAGE_SIZE,
      channelType,
    }),
    queryFn: () =>
      listCodexAuthFiles(channelId, {
        provider: runtimeProviderFilter(channelType),
        page: authPage,
        pageSize: AUTH_FILES_PAGE_SIZE,
        channelType,
      }),
  })

  const quotaQuery = useQuery({
    queryKey: codexQuotaQueryKey(channelId, {
      page: quotaPage,
      pageSize: QUOTA_PAGE_SIZE,
      channelType,
    }),
    queryFn: () =>
      listCodexQuota(channelId, { page: quotaPage, pageSize: QUOTA_PAGE_SIZE, channelType }),
  })

  const modelsQuery = useQuery({
    queryKey: ["codex-auth-models", channelId, modelsDialogFile?.name],
    queryFn: () => getCodexAuthFileModels(channelId, modelsDialogFile?.name || "", channelType),
    enabled: !!modelsDialogFile,
  })

  const toggleMut = useMutation({
    mutationFn: (input: { name: string; disabled: boolean }) =>
      patchCodexAuthFileStatus(channelId, input, channelType),
    onSuccess: () => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.statusUpdated"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (name: string) => deleteCodexAuthFile(channelId, { name }, channelType),
    onSuccess: () => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.authDeleted"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const syncMut = useMutation({
    mutationFn: () => syncCodexKeys(channelId, channelType),
    onSuccess: (res) => {
      for (const queryKey of codexUploadRefreshQueryKeys(channelId)) {
        void queryClient.invalidateQueries({ queryKey })
      }
      toast.success(t("codex.syncSuccess", { count: res.data.synced }))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const uploadMut = useMutation({
    mutationFn: (files: File[]) => uploadCodexAuthFile(channelId, files, channelType),
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
      const res = await startCodexOAuth(channelId, channelType)
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
          const statusRes = await getCodexOAuthStatus(channelId, state, channelType)
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
  }, [channelId, channelType, queryClient, stopPolling, t])

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
  const authTotal = authQuery.data?.data.total ?? 0
  const quotaTotal = quotaQuery.data?.data.total ?? 0
  const authTotalPages = Math.max(1, Math.ceil(authTotal / AUTH_FILES_PAGE_SIZE))
  const quotaTotalPages = Math.max(1, Math.ceil(quotaTotal / QUOTA_PAGE_SIZE))

  useEffect(() => {
    if (authPage > authTotalPages) {
      setAuthPage(authTotalPages)
    }
  }, [authPage, authTotalPages])

  useEffect(() => {
    if (quotaPage > quotaTotalPages) {
      setQuotaPage(quotaTotalPages)
    }
  }, [quotaPage, quotaTotalPages])

  const handleRefresh = useCallback(() => {
    if (modelCount === 0 && authFiles.length > 0) {
      syncMut.mutate()
      return
    }
    void queryClient.invalidateQueries({ queryKey: codexAuthFilesQueryKey(channelId) })
    void queryClient.invalidateQueries({ queryKey: codexQuotaQueryKey(channelId) })
    void queryClient.invalidateQueries({ queryKey: channelsQueryKey })
  }, [authFiles.length, channelId, modelCount, queryClient, syncMut])

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

  const hasCodexQuotaWindows = (item: (typeof quotaItems)[number]) =>
    item.weekly.limitWindowSeconds > 0 || item.codeReview.limitWindowSeconds > 0

  return (
    <div className="mt-2 space-y-3">
      {/* Auth Files Section */}
      <div className="space-y-2">
        <div className="flex flex-col gap-2 lg:flex-row lg:items-center lg:justify-between">
          <h5 className="text-muted-foreground shrink-0 text-xs font-medium tracking-wide whitespace-nowrap uppercase">
            {t("runtime.authFiles", { provider: providerLabel })}
          </h5>
          <div className="flex flex-wrap items-center gap-1 lg:justify-end">
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="h-7 text-xs"
              onClick={handleRefresh}
              disabled={syncMut.isPending}
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
            {t("runtime.noAuthFiles", { provider: providerLabel })}
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
            <RuntimePagination
              currentPage={authPage}
              pageSize={AUTH_FILES_PAGE_SIZE}
              total={authTotal}
              onPageChange={setAuthPage}
            />
          </div>
        )}
      </div>

      {/* Quota Section */}
      {quotaItems.length > 0 && (
        <div className="space-y-2">
          <h5 className="text-muted-foreground text-xs font-medium tracking-wide uppercase">
            {t("runtime.remainingQuota", { provider: providerLabel })}
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
                ) : item.snapshots && item.snapshots.length > 0 ? (
                  <div className="space-y-1.5">
                    {item.snapshots.map((snapshot) => (
                      <div key={snapshot.id}>
                        <div className="mb-0.5 flex items-center justify-between text-[10px]">
                          <span>{snapshot.label}</span>
                          <span>
                            {snapshot.unlimited
                              ? t("runtime.unlimited")
                              : `${Math.max(0, Math.min(snapshot.percentRemaining, 100)).toFixed(1)}%`}
                          </span>
                        </div>
                        <Progress
                          value={
                            snapshot.unlimited
                              ? 100
                              : Math.max(0, Math.min(snapshot.percentRemaining, 100))
                          }
                          className="h-1.5"
                        />
                      </div>
                    ))}
                    {item.resetAt ? (
                      <p className="text-muted-foreground text-[10px]">
                        {t("runtime.resetAt", { resetAt: item.resetAt })}
                      </p>
                    ) : null}
                  </div>
                ) : hasCodexQuotaWindows(item) ? (
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
                ) : (
                  <p className="text-muted-foreground text-[10px]">
                    {t("runtime.quotaUnavailable")}
                  </p>
                )}
              </div>
            ))}
          </div>
          <RuntimePagination
            currentPage={quotaPage}
            pageSize={QUOTA_PAGE_SIZE}
            total={quotaTotal}
            onPageChange={setQuotaPage}
          />
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

function RuntimePagination({
  currentPage,
  pageSize,
  total,
  onPageChange,
}: {
  currentPage: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  const { t } = useTranslation(["model", "common"])

  if (total <= 0) {
    return null
  }

  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const from = (currentPage - 1) * pageSize + 1
  const to = Math.min(total, currentPage * pageSize)

  return (
    <div className="flex flex-col gap-2 pt-1 text-xs sm:flex-row sm:items-center sm:justify-between">
      <div className="text-muted-foreground flex flex-col gap-1 sm:flex-row sm:items-center sm:gap-3">
        <span>{t("pagination.showing", { ns: "common", from, to, total })}</span>
        <span>
          {t("pagination.page", { ns: "common", current: currentPage, total: totalPages })}
        </span>
      </div>
      <div className="flex items-center gap-1 self-end sm:self-auto">
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 text-xs"
          onClick={() => onPageChange(Math.max(1, currentPage - 1))}
          disabled={currentPage <= 1}
        >
          {t("pagination.previous", { ns: "common" })}
        </Button>
        <Button
          type="button"
          variant="outline"
          size="sm"
          className="h-7 text-xs"
          onClick={() => onPageChange(Math.min(totalPages, currentPage + 1))}
          disabled={currentPage >= totalPages}
        >
          {t("pagination.next", { ns: "common" })}
        </Button>
      </div>
    </div>
  )
}

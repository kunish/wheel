import type { Dispatch, SetStateAction } from "react"
import { useQuery, useQueryClient } from "@tanstack/react-query"
import {
  Check,
  Copy,
  Download,
  ExternalLink,
  Eye,
  EyeOff,
  Loader2,
  LogIn,
  Upload,
  X,
  XCircle,
} from "lucide-react"
import { useCallback, useEffect, useMemo, useRef, useState } from "react"

import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
import { ModelSourceBadge } from "@/components/model-source-badge"
import { ProviderIcon } from "@/components/provider-icon"
import { Button } from "@/components/ui/button"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Textarea } from "@/components/ui/textarea"
import {
  createChannel,
  getCodexAuthUploadToastState,
  getCodexOAuthStatus,
  listChannels,
  listCodexAuthFiles,
  runtimeProviderFilter,
  startCodexOAuth,
  uploadCodexAuthFile,
} from "@/lib/api"
import { fetchChannelModelsPreview } from "@/lib/api-client"
import {
  adaptChannelDraftForType,
  ensureCodexChannelId,
  getRuntimeProviderKey,
  isRuntimeChannelType,
  mergeRuntimeChannelModels,
  shouldShowGenericModelFetch,
} from "./codex-channel-draft"
import { channelsQueryKey, codexUploadRefreshQueryKeys } from "./codex-query-keys"

export interface ChannelFormData {
  id?: number
  name: string
  type: number
  enabled: boolean
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  model: string[]
  fetchedModel: string[]
  customModel: string
  paramOverride: string
}

export const EMPTY_CHANNEL_FORM: ChannelFormData = {
  name: "",
  type: 1,
  enabled: true,
  baseUrls: [{ url: "", delay: 0 }],
  keys: [{ channelKey: "", remark: "" }],
  model: [],
  fetchedModel: [],
  customModel: "",
  paramOverride: "",
}

// ─── Model Tag Input ────────────────────────────

function ModelTagInput({
  value,
  onChange,
  fetchedModels,
}: {
  value: string[]
  onChange: (value: string[]) => void
  fetchedModels?: string[]
}) {
  const { t } = useTranslation("model")
  const [input, setInput] = useState("")
  const tags = value ?? []
  const fetchedSet = useMemo(() => new Set(fetchedModels ?? []), [fetchedModels])

  function addTags(raw: string) {
    const newTags = raw
      .split(/[,\n]/)
      .map((t) => t.trim())
      .filter(Boolean)
    if (newTags.length === 0) return
    const merged = [...new Set([...tags, ...newTags])]
    onChange(merged)
    setInput("")
  }

  function removeTag(tag: string) {
    onChange(tags.filter((t) => t !== tag))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault()
      addTags(input)
    }
    if (e.key === "Backspace" && input === "" && tags.length > 0) {
      removeTag(tags[tags.length - 1])
    }
  }

  function handlePaste(e: React.ClipboardEvent<HTMLInputElement>) {
    e.preventDefault()
    addTags(e.clipboardData.getData("text"))
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          onBlur={() => {
            if (input.trim()) addTags(input)
          }}
          placeholder={t("channelDialog.modelInputPlaceholder")}
          className="flex-1"
        />
        <span className="text-muted-foreground text-xs whitespace-nowrap">
          {t("modelCount", { count: tags.length })}
        </span>
      </div>
      {tags.length > 0 && (
        <div className="max-h-40 overflow-y-auto">
          <GroupedModelList
            models={tags}
            renderModel={(tag) => (
              <ModelCard key={tag} modelId={tag} onRemove={() => removeTag(tag)}>
                <ModelSourceBadge modelId={tag} isApiFetched={fetchedSet.has(tag)} />
              </ModelCard>
            )}
          />
        </div>
      )}
    </div>
  )
}

// ─── Fetch Models Button ────────────────────────

function FetchModelsButton({
  form,
  setForm,
}: {
  form: ChannelFormData
  setForm: (f: ChannelFormData) => void
}) {
  const { t } = useTranslation("model")
  const [loading, setLoading] = useState(false)

  const baseUrl = form.baseUrls[0]?.url?.trim()
  const key = form.keys[0]?.channelKey?.trim()
  const canFetch = !!baseUrl && !!key

  async function handleFetch() {
    if (!canFetch) {
      toast.error(t("channelDialog.fetchFillFirst"))
      return
    }
    setLoading(true)
    try {
      const res = await fetchChannelModelsPreview({
        type: form.type,
        baseUrl,
        key,
      })
      const models = res.data.models
      const isFallback = res.data.isFallback === true
      if (models.length === 0) {
        toast.info(t("channelDialog.fetchNoModels"))
        return
      }
      setForm({ ...form, model: models, fetchedModel: isFallback ? [] : models })
      if (isFallback) {
        toast.info(t("channelDialog.fetchFallback", { count: models.length }))
      } else {
        toast.success(t("channelDialog.fetchSuccess", { count: models.length }))
      }
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("channelDialog.fetchError"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button variant="outline" size="sm" onClick={handleFetch} disabled={!canFetch || loading}>
      {loading ? (
        <Loader2 className="mr-1 h-3 w-3 animate-spin" />
      ) : (
        <Download className="mr-1 h-3 w-3" />
      )}
      {loading ? t("channelDialog.fetching") : t("channelDialog.fetchModels")}
    </Button>
  )
}

// ─── Codex OAuth Import Button + Native OAuth Dialog ──────────────────

function CodexOAuthButton({
  channelId,
  channelType,
  ensureChannelId,
  onRuntimeChannelHydrated,
  buttonLabel,
  dialogTitle,
  dialogHint,
  saveChannelFirstMessage,
}: {
  channelId?: number
  channelType?: number
  ensureChannelId?: () => Promise<number>
  onRuntimeChannelHydrated?: (channelId: number) => Promise<void>
  buttonLabel: string
  dialogTitle: string
  dialogHint: string
  saveChannelFirstMessage: string
}) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const capabilitiesQuery = useQuery({
    queryKey: ["codex-auth-capabilities", channelId],
    queryFn: () =>
      listCodexAuthFiles(channelId!, {
        provider: runtimeProviderFilter(channelType),
        page: 1,
        pageSize: 1,
        channelType,
      }),
    enabled: !!channelId,
  })
  const [panelOpen, setPanelOpen] = useState(false)
  const [oauthUrl, setOauthUrl] = useState("")
  const [_oauthState, setOauthState] = useState("")
  const [oauthStatus, setOauthStatus] = useState<
    "idle" | "starting" | "waiting" | "success" | "error"
  >("idle")
  const [oauthError, setOauthError] = useState("")
  const pollRef = useRef<ReturnType<typeof setInterval> | null>(null)

  const stopPolling = useCallback(() => {
    if (pollRef.current) {
      clearInterval(pollRef.current)
      pollRef.current = null
    }
  }, [])

  useEffect(() => {
    return () => stopPolling()
  }, [stopPolling])

  const handleStartOAuth = useCallback(
    async (resolvedChannelId: number) => {
      setOauthStatus("starting")
      setOauthError("")
      setOauthUrl("")
      setOauthState("")
      try {
        const res = await startCodexOAuth(resolvedChannelId, channelType)
        const { url, state } = res.data
        setOauthUrl(url)
        setOauthState(state)
        setOauthStatus("waiting")

        const startTime = Date.now()
        pollRef.current = setInterval(async () => {
          if (Date.now() - startTime > 5 * 60 * 1000) {
            stopPolling()
            setOauthStatus("error")
            setOauthError(t("codex.oauthTimeout"))
            return
          }
          try {
            const statusRes = await getCodexOAuthStatus(resolvedChannelId, state, channelType)
            const { status, error } = statusRes.data
            if (status === "ok") {
              stopPolling()
              setOauthStatus("success")
              for (const queryKey of codexUploadRefreshQueryKeys(resolvedChannelId)) {
                void queryClient.invalidateQueries({ queryKey })
              }
              await onRuntimeChannelHydrated?.(resolvedChannelId)
            } else if (status === "error") {
              stopPolling()
              setOauthStatus("error")
              setOauthError(error || "Unknown error")
            }
          } catch {
            // Network error — keep trying
          }
        }, 3000)
      } catch (err) {
        setOauthStatus("error")
        setOauthError(err instanceof Error ? err.message : "Failed to start OAuth")
      }
    },
    [channelType, onRuntimeChannelHydrated, queryClient, stopPolling, t],
  )

  const handleDialogChange = useCallback(
    (open: boolean) => {
      if (!open) {
        stopPolling()
        setOauthStatus("idle")
        setOauthUrl("")
        setOauthState("")
        setOauthError("")
      }
      setPanelOpen(open)
    },
    [stopPolling],
  )

  async function handleClick() {
    let resolvedChannelId = channelId
    if (!resolvedChannelId && ensureChannelId) {
      try {
        resolvedChannelId = await ensureChannelId()
      } catch (err) {
        toast.error(err instanceof Error ? err.message : saveChannelFirstMessage)
        return
      }
    }
    if (!resolvedChannelId) {
      toast.info(saveChannelFirstMessage)
      return
    }
    setPanelOpen(true)
    void handleStartOAuth(resolvedChannelId)
  }

  const oauthEnabled = capabilitiesQuery.data?.data.capabilities.oauthEnabled
  if (channelId && (capabilitiesQuery.isLoading || oauthEnabled === false)) {
    return null
  }

  return (
    <>
      <Button type="button" variant="outline" size="sm" className="w-fit" onClick={handleClick}>
        <LogIn className="mr-1.5 h-3.5 w-3.5" />
        {buttonLabel}
      </Button>
      <Dialog open={panelOpen} onOpenChange={handleDialogChange}>
        <DialogContent className="max-w-md">
          <DialogHeader>
            <DialogTitle>{dialogTitle}</DialogTitle>
          </DialogHeader>
          <div className="space-y-4">
            <p className="text-muted-foreground text-xs">{dialogHint}</p>

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
    </>
  )
}

function CodexAuthFileUploadButton({
  channelId,
  channelType,
  ensureChannelId,
  onRuntimeChannelHydrated,
  buttonLabel,
  invalidJsonMessage,
  saveChannelFirstMessage,
}: {
  channelId?: number
  channelType?: number
  ensureChannelId?: () => Promise<number>
  onRuntimeChannelHydrated?: (channelId: number) => Promise<void>
  buttonLabel: string
  invalidJsonMessage: string
  saveChannelFirstMessage: string
}) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const fileInputRef = useRef<HTMLInputElement | null>(null)
  const [uploading, setUploading] = useState(false)

  const handleUploadFile = useCallback(
    async (fileList: FileList | null | undefined) => {
      const files = Array.from(fileList ?? [])
      if (files.length === 0) {
        return
      }
      if (files.some((file) => !file.name.toLowerCase().endsWith(".json"))) {
        toast.error(invalidJsonMessage)
        return
      }

      setUploading(true)
      try {
        let resolvedChannelId = channelId
        if (!resolvedChannelId && ensureChannelId) {
          resolvedChannelId = await ensureChannelId()
        }
        if (!resolvedChannelId) {
          toast.info(saveChannelFirstMessage)
          return
        }

        const res = await uploadCodexAuthFile(resolvedChannelId, files, channelType)
        if (res.data.successCount > 0) {
          for (const queryKey of codexUploadRefreshQueryKeys(resolvedChannelId)) {
            void queryClient.invalidateQueries({ queryKey })
          }
          await onRuntimeChannelHydrated?.(resolvedChannelId)
        }

        const toastState = getCodexAuthUploadToastState(res.data)
        toast[toastState.level](t(toastState.key, toastState.values))
      } catch (err) {
        toast.error(err instanceof Error ? err.message : invalidJsonMessage)
      } finally {
        setUploading(false)
      }
    },
    [
      channelId,
      channelType,
      ensureChannelId,
      invalidJsonMessage,
      onRuntimeChannelHydrated,
      queryClient,
      saveChannelFirstMessage,
      t,
    ],
  )

  return (
    <>
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="w-fit"
        onClick={() => fileInputRef.current?.click()}
        disabled={uploading}
      >
        {uploading ? (
          <Loader2 className="mr-1.5 h-3.5 w-3.5 animate-spin" />
        ) : (
          <Upload className="mr-1.5 h-3.5 w-3.5" />
        )}
        {uploading ? t("codex.uploadingFile") : buttonLabel}
      </Button>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        accept=".json"
        className="hidden"
        onChange={(e) => {
          void handleUploadFile(e.target.files)
          e.currentTarget.value = ""
        }}
      />
    </>
  )
}

// ─── Channel Dialog ────────────────────────────

interface ChannelDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: ChannelFormData
  setForm: Dispatch<SetStateAction<ChannelFormData>>
  onSave: () => void
  isPending: boolean
}

export default function ChannelDialog({
  open,
  onOpenChange,
  form,
  setForm,
  onSave,
  isPending,
}: ChannelDialogProps) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const [showKey, setShowKey] = useState(false)
  const pendingCodexChannelIdRef = useRef<Promise<number> | null>(null)
  const runtimeProviderKey = getRuntimeProviderKey(form.type)
  const isRuntimeChannel = isRuntimeChannelType(form.type)
  const runtimeProviderLabel = runtimeProviderKey ? t(`typeLabels.${form.type}`) : ""
  const runtimeTitle = runtimeProviderLabel
  const runtimeDescription = runtimeProviderKey
    ? t(`runtime.providerDescription.${runtimeProviderKey}`, { provider: runtimeProviderLabel })
    : ""
  const runtimeAuthLabel = t("runtime.credentialsTitle", { provider: runtimeProviderLabel })
  const runtimeKeyHint = t("runtime.keyManagedByAuthFiles", { provider: runtimeProviderLabel })
  const runtimeModelHint = t("runtime.modelHint", { provider: runtimeProviderLabel })
  const runtimeImportOAuthLabel = t("runtime.importOAuth", { provider: runtimeProviderLabel })
  const runtimeImportFileLabel = t("runtime.importFile", { provider: runtimeProviderLabel })
  const runtimeSaveChannelFirst = t("runtime.saveChannelFirst", { provider: runtimeProviderLabel })
  const runtimeOauthHint = t("runtime.oauthHint", { provider: runtimeProviderLabel })
  const runtimeInvalidJsonMessage = t("runtime.invalidJsonFile", { provider: runtimeProviderLabel })

  const ensureChannelIdForCodex = useCallback(async () => {
    if (form.id) {
      return form.id
    }
    if (!pendingCodexChannelIdRef.current) {
      pendingCodexChannelIdRef.current = (async () => {
        const channelId = await ensureCodexChannelId({
          form,
          saveChannel: createChannel,
        })
        setForm({ ...form, id: channelId })
        await queryClient.invalidateQueries({ queryKey: channelsQueryKey })
        toast.success(t("toast.channelCreated"))
        return channelId
      })().finally(() => {
        pendingCodexChannelIdRef.current = null
      })
    }

    return pendingCodexChannelIdRef.current
  }, [form, queryClient, setForm, t])

  const hydrateRuntimeChannelModels = useCallback(
    async (channelId: number) => {
      await queryClient.invalidateQueries({ queryKey: channelsQueryKey })
      const refreshed = await queryClient.fetchQuery({
        queryKey: channelsQueryKey,
        queryFn: listChannels,
      })
      const channel = (refreshed.data?.channels as ChannelFormData[] | undefined)?.find(
        (item) => item.id === channelId,
      )
      if (!channel) {
        return
      }
      setForm((current) => mergeRuntimeChannelModels(current, channel))
    },
    [queryClient, setForm],
  )

  const typeLabels = useMemo(
    () => ({
      1: t("typeLabels.1"),
      2: t("typeLabels.2"),
      3: t("typeLabels.3"),
      4: t("typeLabels.4"),
      5: t("typeLabels.5"),
      10: t("typeLabels.10"),
      11: t("typeLabels.11"),
      12: t("typeLabels.12"),
      13: t("typeLabels.13"),
      20: t("typeLabels.20"),
      21: t("typeLabels.21"),
      22: t("typeLabels.22"),
      23: t("typeLabels.23"),
      24: t("typeLabels.24"),
      25: t("typeLabels.25"),
      26: t("typeLabels.26"),
      27: t("typeLabels.27"),
      28: t("typeLabels.28"),
      29: t("typeLabels.29"),
      30: t("typeLabels.30"),
      31: t("typeLabels.31"),
      32: t("typeLabels.32"),
      33: t("typeLabels.33"),
      34: t("typeLabels.34"),
    }),
    [t],
  )

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] w-full max-w-2xl max-w-[95vw] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {form.id ? t("channelDialog.editTitle") : t("channelDialog.createTitle")}
          </DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label>{t("channelDialog.name")}</Label>
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("channelDialog.providerType")}</Label>
            <Select
              value={String(form.type)}
              onValueChange={(v) => {
                setForm(adaptChannelDraftForType(form, Number(v)))
              }}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(typeLabels).map(([val, label]) => (
                  <SelectItem key={val} value={val}>
                    <span className="inline-flex items-center gap-1.5">
                      <ProviderIcon channelType={Number(val)} size={14} />
                      {label}
                    </span>
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {isRuntimeChannel ? (
            <div className="rounded-lg border border-dashed px-3 py-3">
              <div className="space-y-1">
                <p className="text-sm font-medium">{runtimeTitle}</p>
                <p className="text-muted-foreground text-xs">{runtimeDescription}</p>
              </div>
            </div>
          ) : null}

          {!isRuntimeChannel ? (
            <div className="flex flex-col gap-2">
              <Label>{t("channelDialog.baseUrl")}</Label>
              <Input
                placeholder="https://api.openai.com"
                value={form.baseUrls[0]?.url ?? ""}
                onChange={(e) =>
                  setForm({
                    ...form,
                    baseUrls: [{ url: e.target.value, delay: form.baseUrls[0]?.delay ?? 0 }],
                  })
                }
              />
            </div>
          ) : null}

          {isRuntimeChannel ? (
            <div className="flex flex-col gap-2">
              <Label>{runtimeAuthLabel}</Label>
              <p className="text-muted-foreground text-xs">{runtimeKeyHint}</p>
              <div className="flex flex-wrap gap-2">
                <CodexOAuthButton
                  channelId={form.id}
                  channelType={form.type}
                  ensureChannelId={ensureChannelIdForCodex}
                  onRuntimeChannelHydrated={hydrateRuntimeChannelModels}
                  buttonLabel={runtimeImportOAuthLabel}
                  dialogTitle={runtimeImportOAuthLabel}
                  dialogHint={runtimeOauthHint}
                  saveChannelFirstMessage={runtimeSaveChannelFirst}
                />
                <CodexAuthFileUploadButton
                  channelId={form.id}
                  channelType={form.type}
                  ensureChannelId={ensureChannelIdForCodex}
                  onRuntimeChannelHydrated={hydrateRuntimeChannelModels}
                  buttonLabel={runtimeImportFileLabel}
                  invalidJsonMessage={runtimeInvalidJsonMessage}
                  saveChannelFirstMessage={runtimeSaveChannelFirst}
                />
              </div>
            </div>
          ) : (
            <div className="flex flex-col gap-2">
              <Label>{t("channelDialog.apiKey")}</Label>
              <div className="relative">
                <Input
                  type={showKey ? "text" : "password"}
                  placeholder="sk-..."
                  value={form.keys[0]?.channelKey ?? ""}
                  onChange={(e) =>
                    setForm({
                      ...form,
                      keys: [{ channelKey: e.target.value, remark: form.keys[0]?.remark ?? "" }],
                    })
                  }
                  className="pr-9"
                />
                <Button
                  type="button"
                  variant="ghost"
                  size="icon"
                  className="absolute top-1/2 right-1 h-7 w-7 -translate-y-1/2"
                  onClick={() => setShowKey(!showKey)}
                  aria-label={
                    showKey ? t("channelDialog.hideApiKey") : t("channelDialog.showApiKey")
                  }
                >
                  {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                </Button>
              </div>
            </div>
          )}

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>{t("channelDialog.models")}</Label>
              <div className="flex items-center gap-1">
                {form.model.length > 0 && (
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={() => setForm({ ...form, model: [], fetchedModel: [] })}
                  >
                    <X className="mr-1 h-3 w-3" />
                    {t("clearAll", { ns: "model" })}
                  </Button>
                )}
                {shouldShowGenericModelFetch(form.type) ? (
                  <FetchModelsButton form={form} setForm={setForm} />
                ) : null}
              </div>
            </div>
            {isRuntimeChannel ? (
              <p className="text-muted-foreground text-xs">{runtimeModelHint}</p>
            ) : null}
            <ModelTagInput
              value={form.model}
              onChange={(model) => setForm({ ...form, model })}
              fetchedModels={form.fetchedModel}
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("channelDialog.paramOverride")}</Label>
            <Textarea
              value={form.paramOverride}
              onChange={(e) => setForm({ ...form, paramOverride: e.target.value })}
              placeholder={t("channelDialog.paramOverridePlaceholder")}
              rows={3}
            />
          </div>

          <Button className="mt-2" onClick={onSave} disabled={isPending || !form.name}>
            {isPending ? t("channelDialog.saving") : t("actions.save", { ns: "common" })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

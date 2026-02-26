import type { ApiKeyRecord } from "@/lib/api-client"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Pencil, Plus, Trash2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { lazy, Suspense, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  changePassword,
  changeUsername,
  createApiKey,
  deleteApiKey,
  listApiKeys,
  updateApiKey,
} from "@/lib/api-client"

// ───────────── Lazy-loaded sections ─────────────

const SystemConfigSection = lazy(() => import("./settings/system-config-section"))

const BackupSection = lazy(() => import("./settings/backup-section"))

const ConnectionSection = lazy(() => import("./settings/connection-section"))

const VersionSection = lazy(() => import("./settings/version-section"))

// ───────────── API Keys types ─────────────

interface ApiKeyFormData {
  name: string
  expireAt: string
  maxCost: string
  supportedModels: string
  rpmLimit: string
  tpmLimit: string
}

const EMPTY_FORM: ApiKeyFormData = {
  name: "",
  expireAt: "",
  maxCost: "",
  supportedModels: "",
  rpmLimit: "",
  tpmLimit: "",
}

// ───────────── Account Section ─────────────

function AccountSection() {
  const { t } = useTranslation("settings")
  const [newUsername, setNewUsername] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")

  const usernameMutation = useMutation({
    mutationFn: () => changeUsername(newUsername),
    onSuccess: () => {
      toast.success(t("account.usernameUpdated"))
      setNewUsername("")
    },
    onError: () => toast.error(t("account.usernameUpdateFailed")),
  })

  const passwordMutation = useMutation({
    mutationFn: () => changePassword(newPassword),
    onSuccess: () => {
      toast.success(t("account.passwordUpdated"))
      setNewPassword("")
      setConfirmPassword("")
    },
    onError: () => toast.error(t("account.passwordUpdateFailed")),
  })

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>{t("account.changeUsername")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!newUsername.trim()) return
              usernameMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>{t("account.newUsername")}</Label>
              <Input
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder={t("account.enterNewUsername")}
                required
              />
            </div>
            <Button type="submit" disabled={usernameMutation.isPending}>
              {t("account.updateUsername")}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("account.changePassword")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (newPassword.length < 8) {
                toast.error(t("account.passwordMinLength"))
                return
              }
              if (newPassword !== confirmPassword) {
                toast.error(t("account.passwordsDoNotMatch"))
                return
              }
              if (!newPassword) return
              passwordMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>{t("account.newPassword")}</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder={t("account.enterNewPassword")}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("account.confirmPassword")}</Label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder={t("account.confirmNewPassword")}
                required
              />
            </div>
            <Button type="submit" disabled={passwordMutation.isPending}>
              {t("account.updatePassword")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

// ───────────── API Keys Section ─────────────

function ApiKeysSection() {
  const { t } = useTranslation("settings")
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [createdKey, setCreatedKey] = useState<string | null>(null)
  const [keyCopied, setKeyCopied] = useState(false)
  const [createForm, setCreateForm] = useState<ApiKeyFormData>(EMPTY_FORM)
  const [editingKey, setEditingKey] = useState<ApiKeyRecord | null>(null)
  const [editForm, setEditForm] = useState<ApiKeyFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<ApiKeyRecord | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["apikeys"],
    queryFn: listApiKeys,
  })

  const createMutation = useMutation({
    mutationFn: (form: ApiKeyFormData) =>
      createApiKey({
        name: form.name,
        expireAt: form.expireAt ? Math.floor(new Date(form.expireAt).getTime() / 1000) : 0,
        maxCost: form.maxCost ? Number.parseFloat(form.maxCost) : 0,
        supportedModels: form.supportedModels,
        rpmLimit: form.rpmLimit ? Number.parseInt(form.rpmLimit, 10) : 0,
        tpmLimit: form.tpmLimit ? Number.parseInt(form.tpmLimit, 10) : 0,
      }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      const key = res.data?.apiKey
      if (key) setCreatedKey(key)
      setCreateForm(EMPTY_FORM)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: ApiKeyFormData }) =>
      updateApiKey({
        id,
        name: form.name,
        expireAt: form.expireAt ? Math.floor(new Date(form.expireAt).getTime() / 1000) : 0,
        maxCost: form.maxCost ? Number.parseFloat(form.maxCost) : 0,
        supportedModels: form.supportedModels,
        rpmLimit: form.rpmLimit ? Number.parseInt(form.rpmLimit, 10) : 0,
        tpmLimit: form.tpmLimit ? Number.parseInt(form.tpmLimit, 10) : 0,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      setEditingKey(null)
      toast.success(t("apiKeys.apiKeyUpdated"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      toast.success(t("apiKeys.apiKeyDeleted"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const apiKeys = data?.data?.apiKeys ?? []

  function maskKey(key: string) {
    if (key.length <= 16) return key
    return `${key.slice(0, 16)}...${key.slice(-4)}`
  }

  function formatExpiry(timestamp: number) {
    if (!timestamp) return t("apiKeys.never")
    return new Date(timestamp * 1000).toLocaleDateString()
  }

  function timestampToDateInput(timestamp: number): string {
    if (!timestamp) return ""
    return new Date(timestamp * 1000).toISOString().split("T")[0]
  }

  function openEdit(k: ApiKeyRecord) {
    setEditForm({
      name: k.name,
      expireAt: timestampToDateInput(k.expireAt),
      maxCost: k.maxCost ? String(k.maxCost) : "",
      supportedModels: k.supportedModels ?? "",
      rpmLimit: k.rpmLimit ? String(k.rpmLimit) : "",
      tpmLimit: k.tpmLimit ? String(k.tpmLimit) : "",
    })
    setEditingKey(k)
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>{t("apiKeys.title")}</CardTitle>
        <Dialog
          open={showCreate}
          onOpenChange={(open) => {
            if (!open && createdKey && !keyCopied) {
              toast.error(t("apiKeys.copyKeyBeforeClosing"))
              return
            }
            setShowCreate(open)
            if (!open) {
              setCreatedKey(null)
              setKeyCopied(false)
              setCreateForm(EMPTY_FORM)
            }
          }}
        >
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="mr-2 h-4 w-4" /> {t("apiKeys.createKey")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("apiKeys.createApiKey")}</DialogTitle>
            </DialogHeader>
            {createdKey ? (
              <div className="flex flex-col gap-3">
                <p className="text-muted-foreground text-sm">{t("apiKeys.saveKeyWarning")}</p>
                <div className="bg-muted flex items-center gap-2 rounded-md p-3">
                  <code className="flex-1 text-sm break-all">{createdKey}</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    aria-label="Copy API key"
                    onClick={() => {
                      navigator.clipboard.writeText(createdKey)
                      setKeyCopied(true)
                      toast.success(t("apiKeys.copied"))
                    }}
                  >
                    <Copy className="h-4 w-4" />
                  </Button>
                </div>
                <Button
                  onClick={() => {
                    setCreatedKey(null)
                    setShowCreate(false)
                  }}
                >
                  {t("apiKeys.done")}
                </Button>
              </div>
            ) : (
              <ApiKeyForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => createMutation.mutate(createForm)}
                isPending={createMutation.isPending}
                submitLabel={t("actions.create", { ns: "common" })}
              />
            )}
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {/* Edit Dialog */}
        <Dialog
          open={!!editingKey}
          onOpenChange={(open) => {
            if (!open) setEditingKey(null)
          }}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("apiKeys.editApiKey")}</DialogTitle>
            </DialogHeader>
            <ApiKeyForm
              form={editForm}
              onChange={setEditForm}
              onSubmit={() => {
                if (editingKey) {
                  updateMutation.mutate({ id: editingKey.id, form: editForm })
                }
              }}
              isPending={updateMutation.isPending}
              submitLabel={t("actions.save", { ns: "common" })}
            />
          </DialogContent>
        </Dialog>

        {/* Delete Confirmation */}
        <ConfirmDeleteDialog
          open={!!deleteConfirm}
          onOpenChange={(open) => !open && setDeleteConfirm(null)}
          title={t("apiKeys.deleteTitle", { name: deleteConfirm?.name })}
          description={t("apiKeys.deleteDescription")}
          cancelLabel={t("actions.cancel", { ns: "common" })}
          confirmLabel={t("actions.delete", { ns: "common" })}
          onConfirm={() => {
            if (deleteConfirm) deleteMutation.mutate(deleteConfirm.id)
            setDeleteConfirm(null)
          }}
        />

        {isLoading ? (
          <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("apiKeys.table.name")}</TableHead>
                  <TableHead>{t("apiKeys.table.key")}</TableHead>
                  <TableHead>{t("apiKeys.table.status")}</TableHead>
                  <TableHead>{t("apiKeys.table.expires")}</TableHead>
                  <TableHead>{t("apiKeys.table.costLimit")}</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                <AnimatePresence initial={false}>
                  {apiKeys.map((k) => (
                    <motion.tr
                      key={k.id}
                      initial={{ opacity: 0, y: -10 }}
                      animate={{ opacity: 1, y: 0 }}
                      exit={{ opacity: 0, y: -10 }}
                      transition={{ duration: 0.2 }}
                      className="border-b"
                    >
                      <TableCell className="font-medium">{k.name}</TableCell>
                      <TableCell>
                        <div className="flex items-center gap-1">
                          <code className="text-xs">{maskKey(k.apiKey)}</code>
                          <Button
                            variant="ghost"
                            size="icon"
                            className="h-6 w-6"
                            aria-label={`Copy API key ${k.name}`}
                            onClick={() => {
                              navigator.clipboard.writeText(k.apiKey)
                              toast.success(t("apiKeys.copied"))
                            }}
                          >
                            <Copy className="h-3 w-3" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={k.enabled ? "default" : "secondary"}>
                          {k.enabled ? t("apiKeys.statusActive") : t("apiKeys.statusDisabled")}
                        </Badge>
                      </TableCell>
                      <TableCell>{formatExpiry(k.expireAt)}</TableCell>
                      <TableCell>
                        ${k.totalCost.toFixed(4)}
                        {k.maxCost > 0 && (
                          <span className="text-muted-foreground"> / ${k.maxCost.toFixed(2)}</span>
                        )}
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Edit API key"
                            onClick={() => openEdit(k)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Delete API key"
                            onClick={() => setDeleteConfirm(k)}
                          >
                            <Trash2 className="text-destructive h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </motion.tr>
                  ))}
                </AnimatePresence>
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  )
}

function ApiKeyForm({
  form,
  onChange,
  onSubmit,
  isPending,
  submitLabel,
}: {
  form: ApiKeyFormData
  onChange: (f: ApiKeyFormData) => void
  onSubmit: () => void
  isPending: boolean
  submitLabel: string
}) {
  const { t } = useTranslation("settings")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>{t("apiKeys.form.name")}</Label>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder={t("apiKeys.form.namePlaceholder")}
          required
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("apiKeys.form.expireDate")}</Label>
        <Input
          type="date"
          value={form.expireAt}
          onChange={(e) => onChange({ ...form, expireAt: e.target.value })}
        />
        <p className="text-muted-foreground text-xs">{t("apiKeys.form.expireDateHint")}</p>
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("apiKeys.form.costLimit")}</Label>
        <Input
          type="number"
          step="0.01"
          min="0"
          value={form.maxCost}
          onChange={(e) => onChange({ ...form, maxCost: e.target.value })}
          placeholder={t("apiKeys.form.costLimitPlaceholder")}
        />
        <p className="text-muted-foreground text-xs">{t("apiKeys.form.costLimitHint")}</p>
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("apiKeys.form.modelWhitelist")}</Label>
        <Input
          value={form.supportedModels}
          onChange={(e) => onChange({ ...form, supportedModels: e.target.value })}
          placeholder={t("apiKeys.form.modelWhitelistPlaceholder")}
        />
        <p className="text-muted-foreground text-xs">{t("apiKeys.form.modelWhitelistHint")}</p>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label>{t("apiKeys.form.rpmLimit")}</Label>
          <Input
            type="number"
            min="0"
            value={form.rpmLimit}
            onChange={(e) => onChange({ ...form, rpmLimit: e.target.value })}
            placeholder={t("apiKeys.form.rpmLimitPlaceholder")}
          />
          <p className="text-muted-foreground text-xs">{t("apiKeys.form.rpmLimitHint")}</p>
        </div>
        <div className="flex flex-col gap-2">
          <Label>{t("apiKeys.form.tpmLimit")}</Label>
          <Input
            type="number"
            min="0"
            value={form.tpmLimit}
            onChange={(e) => onChange({ ...form, tpmLimit: e.target.value })}
            placeholder={t("apiKeys.form.tpmLimitPlaceholder")}
          />
          <p className="text-muted-foreground text-xs">{t("apiKeys.form.tpmLimitHint")}</p>
        </div>
      </div>
      <Button type="submit" disabled={isPending}>
        {submitLabel}
      </Button>
    </form>
  )
}

// ───────────── Settings Page ─────────────

export default function SettingsPage() {
  const { t } = useTranslation("settings")
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <h2 className="shrink-0 pb-4 text-2xl font-bold tracking-tight">{t("title")}</h2>
      <div className="min-h-0 flex-1 space-y-6 overflow-auto">
        <ApiKeysSection />
        <AccountSection />
        <Suspense
          fallback={
            <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
          }
        >
          <SystemConfigSection />
        </Suspense>
        <Suspense
          fallback={
            <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
          }
        >
          <ConnectionSection />
        </Suspense>
        <Suspense
          fallback={
            <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
          }
        >
          <BackupSection />
        </Suspense>
        <Suspense
          fallback={
            <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
          }
        >
          <VersionSection />
        </Suspense>
      </div>
    </div>
  )
}

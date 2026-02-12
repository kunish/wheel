import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Pencil, Plus, Trash2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { lazy, Suspense, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from "@/components/ui/alert-dialog"
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
} from "@/lib/api"

// ───────────── Lazy-loaded sections ─────────────

const SystemConfigSection = lazy(() => import("./settings/system-config-section"))

const BackupSection = lazy(() => import("./settings/backup-section"))

// ───────────── API Keys types ─────────────

interface ApiKeyRecord {
  id: number
  name: string
  apiKey: string
  enabled: boolean
  expireAt: number
  maxCost: number
  totalCost: number
  supportedModels: string
}

interface ApiKeyFormData {
  name: string
  expireAt: string
  maxCost: string
  supportedModels: string
}

const EMPTY_FORM: ApiKeyFormData = {
  name: "",
  expireAt: "",
  maxCost: "",
  supportedModels: "",
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
      }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      const key = (res.data as { apiKey?: string })?.apiKey
      if (key) setCreatedKey(key)
      setCreateForm(EMPTY_FORM)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: ApiKeyFormData }) =>
      updateApiKey({
        id,
        name: form.name,
        expireAt: form.expireAt ? Math.floor(new Date(form.expireAt).getTime() / 1000) : 0,
        maxCost: form.maxCost ? Number.parseFloat(form.maxCost) : 0,
        supportedModels: form.supportedModels,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      setEditingKey(null)
      toast.success(t("apiKeys.apiKeyUpdated"))
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      toast.success(t("apiKeys.apiKeyDeleted"))
    },
  })

  const apiKeys = (data?.data?.apiKeys ?? []) as ApiKeyRecord[]

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
        <AlertDialog
          open={!!deleteConfirm}
          onOpenChange={(open) => !open && setDeleteConfirm(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("apiKeys.deleteTitle", { name: deleteConfirm?.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>{t("apiKeys.deleteDescription")}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("actions.cancel", { ns: "common" })}</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={() => {
                  if (deleteConfirm) deleteMutation.mutate(deleteConfirm.id)
                  setDeleteConfirm(null)
                }}
              >
                {t("actions.delete", { ns: "common" })}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

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
                          <Button variant="ghost" size="icon" onClick={() => openEdit(k)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => setDeleteConfirm(k)}>
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
          <BackupSection />
        </Suspense>
      </div>
    </div>
  )
}

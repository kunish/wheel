import type { ApiKeyRecord } from "@/lib/api-client"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Key, Pencil, Plus, Trash2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent } from "@/components/ui/card"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { FormField } from "@/components/ui/form-field"
import { Input } from "@/components/ui/input"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { createApiKey, deleteApiKey, listApiKeys, updateApiKey } from "@/lib/api-client"

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
  const { t } = useTranslation("keys")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <FormField label={t("form.name")} required>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder={t("form.namePlaceholder")}
          required
        />
      </FormField>
      <FormField label={t("form.expireDate")} hint={t("form.expireDateHint")}>
        <Input
          type="date"
          value={form.expireAt}
          onChange={(e) => onChange({ ...form, expireAt: e.target.value })}
        />
      </FormField>
      <FormField label={t("form.costLimit")} hint={t("form.costLimitHint")}>
        <Input
          type="number"
          step="0.01"
          min="0"
          value={form.maxCost}
          onChange={(e) => onChange({ ...form, maxCost: e.target.value })}
          placeholder={t("form.costLimitPlaceholder")}
        />
      </FormField>
      <FormField label={t("form.modelWhitelist")} hint={t("form.modelWhitelistHint")}>
        <Input
          value={form.supportedModels}
          onChange={(e) => onChange({ ...form, supportedModels: e.target.value })}
          placeholder={t("form.modelWhitelistPlaceholder")}
        />
      </FormField>
      <div className="grid grid-cols-2 gap-4">
        <FormField label={t("form.rpmLimit")} hint={t("form.rpmLimitHint")}>
          <Input
            type="number"
            min="0"
            value={form.rpmLimit}
            onChange={(e) => onChange({ ...form, rpmLimit: e.target.value })}
            placeholder={t("form.rpmLimitPlaceholder")}
          />
        </FormField>
        <FormField label={t("form.tpmLimit")} hint={t("form.tpmLimitHint")}>
          <Input
            type="number"
            min="0"
            value={form.tpmLimit}
            onChange={(e) => onChange({ ...form, tpmLimit: e.target.value })}
            placeholder={t("form.tpmLimitPlaceholder")}
          />
        </FormField>
      </div>
      <Button type="submit" disabled={isPending}>
        {submitLabel}
      </Button>
    </form>
  )
}

export default function KeysPage() {
  const { t } = useTranslation("keys")
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
      toast.success(t("apiKeyUpdated"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      toast.success(t("apiKeyDeleted"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const apiKeys = data?.data?.apiKeys ?? []

  function maskKey(key: string) {
    if (key.length <= 16) return key
    return `${key.slice(0, 16)}...${key.slice(-4)}`
  }

  function formatExpiry(timestamp: number) {
    if (!timestamp) return t("never")
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
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between pb-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
          <p className="text-muted-foreground text-sm">{t("description")}</p>
        </div>
        <Button onClick={() => setShowCreate(true)}>
          <Plus className="mr-1 h-4 w-4" />
          {t("createKey")}
        </Button>
      </div>

      {/* Create Dialog */}
      <Dialog
        open={showCreate}
        onOpenChange={(open) => {
          if (!open && createdKey && !keyCopied) {
            toast.error(t("copyKeyBeforeClosing"))
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
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("createApiKey")}</DialogTitle>
          </DialogHeader>
          {createdKey ? (
            <div className="flex flex-col gap-3">
              <p className="text-muted-foreground text-sm">{t("saveKeyWarning")}</p>
              <div className="bg-muted flex items-center gap-2 rounded-md p-3">
                <code className="flex-1 text-sm break-all">{createdKey}</code>
                <Button
                  variant="ghost"
                  size="icon"
                  aria-label="Copy API key"
                  onClick={() => {
                    navigator.clipboard.writeText(createdKey)
                    setKeyCopied(true)
                    toast.success(t("copied"))
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
                {t("done")}
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

      {/* Edit Dialog */}
      <Dialog
        open={!!editingKey}
        onOpenChange={(open) => {
          if (!open) setEditingKey(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("editApiKey")}</DialogTitle>
          </DialogHeader>
          <ApiKeyForm
            form={editForm}
            onChange={setEditForm}
            onSubmit={() => {
              if (editingKey) updateMutation.mutate({ id: editingKey.id, form: editForm })
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
        title={t("deleteTitle", { name: deleteConfirm?.name })}
        description={t("deleteDescription")}
        cancelLabel={t("actions.cancel", { ns: "common" })}
        confirmLabel={t("actions.delete", { ns: "common" })}
        onConfirm={() => {
          if (deleteConfirm) deleteMutation.mutate(deleteConfirm.id)
          setDeleteConfirm(null)
        }}
      />

      {/* Content */}
      <div className="min-h-0 flex-1 overflow-auto">
        {isLoading ? (
          <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
        ) : apiKeys.length === 0 ? (
          <Card>
            <CardContent className="flex flex-col items-center gap-3 py-12">
              <Key className="text-muted-foreground h-10 w-10" />
              <h3 className="text-lg font-semibold">{t("empty.title")}</h3>
              <p className="text-muted-foreground text-sm">{t("empty.description")}</p>
              <Button onClick={() => setShowCreate(true)} className="mt-2">
                <Plus className="mr-1 h-4 w-4" />
                {t("createKey")}
              </Button>
            </CardContent>
          </Card>
        ) : (
          <div className="overflow-x-auto rounded-lg border">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t("table.name")}</TableHead>
                  <TableHead>{t("table.key")}</TableHead>
                  <TableHead>{t("table.status")}</TableHead>
                  <TableHead>{t("table.expires")}</TableHead>
                  <TableHead>{t("table.costLimit")}</TableHead>
                  <TableHead>{t("table.rateLimit")}</TableHead>
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
                              toast.success(t("copied"))
                            }}
                          >
                            <Copy className="h-3 w-3" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={k.enabled ? "default" : "secondary"}>
                          {k.enabled ? t("statusActive") : t("statusDisabled")}
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
                        <div className="flex flex-col text-xs">
                          {k.rpmLimit > 0 && <span>{k.rpmLimit} RPM</span>}
                          {k.tpmLimit > 0 && <span>{k.tpmLimit} TPM</span>}
                          {!k.rpmLimit && !k.tpmLimit && (
                            <span className="text-muted-foreground">{t("unlimited")}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Edit"
                            onClick={() => openEdit(k)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            aria-label="Delete"
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
      </div>
    </div>
  )
}

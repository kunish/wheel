import type { ModelLimitRecord } from "@/lib/api-client"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Gauge, Pencil, Plus, Trash2 } from "lucide-react"
import { useState } from "react"
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
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  createModelLimit,
  deleteModelLimit,
  listModelLimits,
  updateModelLimit,
} from "@/lib/api-client"

interface LimitFormData {
  model: string
  rpm: string
  tpm: string
  dailyRequests: string
  dailyTokens: string
  enabled: boolean
}

const EMPTY_FORM: LimitFormData = {
  model: "",
  rpm: "",
  tpm: "",
  dailyRequests: "",
  dailyTokens: "",
  enabled: true,
}

export default function ModelLimitsPage() {
  const { t } = useTranslation("model-limits")
  const queryClient = useQueryClient()
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<LimitFormData>(EMPTY_FORM)
  const [editingLimit, setEditingLimit] = useState<ModelLimitRecord | null>(null)
  const [editForm, setEditForm] = useState<LimitFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<ModelLimitRecord | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["model-limits"],
    queryFn: listModelLimits,
  })

  const limits = data?.data?.limits ?? []

  const createMutation = useMutation({
    mutationFn: (form: LimitFormData) =>
      createModelLimit({
        model: form.model,
        rpm: form.rpm ? Number.parseInt(form.rpm, 10) : 0,
        tpm: form.tpm ? Number.parseInt(form.tpm, 10) : 0,
        dailyRequests: form.dailyRequests ? Number.parseInt(form.dailyRequests, 10) : 0,
        dailyTokens: form.dailyTokens ? Number.parseInt(form.dailyTokens, 10) : 0,
        enabled: form.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-limits"] })
      setCreateForm(EMPTY_FORM)
      setShowCreate(false)
      toast.success(t("limitSaved"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: LimitFormData }) =>
      updateModelLimit({
        id,
        model: form.model,
        rpm: form.rpm ? Number.parseInt(form.rpm, 10) : 0,
        tpm: form.tpm ? Number.parseInt(form.tpm, 10) : 0,
        dailyRequests: form.dailyRequests ? Number.parseInt(form.dailyRequests, 10) : 0,
        dailyTokens: form.dailyTokens ? Number.parseInt(form.dailyTokens, 10) : 0,
        enabled: form.enabled,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-limits"] })
      setEditingLimit(null)
      toast.success(t("limitSaved"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteModelLimit,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-limits"] })
      toast.success(t("limitDeleted"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  function openEdit(limit: ModelLimitRecord) {
    setEditForm({
      model: limit.model,
      rpm: limit.rpm ? String(limit.rpm) : "",
      tpm: limit.tpm ? String(limit.tpm) : "",
      dailyRequests: limit.dailyRequests ? String(limit.dailyRequests) : "",
      dailyTokens: limit.dailyTokens ? String(limit.dailyTokens) : "",
      enabled: limit.enabled,
    })
    setEditingLimit(limit)
  }

  function formatLimit(value: number) {
    if (!value) return t("unlimited")
    return value.toLocaleString()
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <h2 className="shrink-0 pb-4 text-2xl font-bold tracking-tight">{t("title")}</h2>
      <p className="text-muted-foreground mb-4 text-sm">{t("description")}</p>

      <Card>
        <CardHeader className="flex flex-row items-center justify-between">
          <CardTitle>{t("title")}</CardTitle>
          <Dialog
            open={showCreate}
            onOpenChange={(open) => {
              setShowCreate(open)
              if (!open) setCreateForm(EMPTY_FORM)
            }}
          >
            <DialogTrigger asChild>
              <Button size="sm" type="button">
                <Plus className="mr-2 h-4 w-4" /> {t("addLimit")}
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("addLimit")}</DialogTitle>
              </DialogHeader>
              <LimitForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => createMutation.mutate(createForm)}
                isPending={createMutation.isPending}
              />
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent>
          {/* Edit Dialog */}
          <Dialog
            open={!!editingLimit}
            onOpenChange={(open) => {
              if (!open) setEditingLimit(null)
            }}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("editLimit")}</DialogTitle>
              </DialogHeader>
              <LimitForm
                form={editForm}
                onChange={setEditForm}
                onSubmit={() => {
                  if (editingLimit) {
                    updateMutation.mutate({ id: editingLimit.id, form: editForm })
                  }
                }}
                isPending={updateMutation.isPending}
              />
            </DialogContent>
          </Dialog>

          {/* Delete Confirmation */}
          <ConfirmDeleteDialog
            open={!!deleteConfirm}
            onOpenChange={(open) => !open && setDeleteConfirm(null)}
            title={t("deleteTitle", { name: deleteConfirm?.model })}
            description={t("deleteDescription")}
            cancelLabel={t("actions.cancel", { ns: "common" })}
            confirmLabel={t("actions.delete", { ns: "common" })}
            onConfirm={() => {
              if (deleteConfirm) deleteMutation.mutate(deleteConfirm.id)
              setDeleteConfirm(null)
            }}
          />

          {isLoading ? (
            <p className="text-muted-foreground py-8 text-center">
              {t("actions.loading", { ns: "common" })}
            </p>
          ) : limits.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-16">
              <Gauge className="text-muted-foreground h-10 w-10" />
              <p className="text-muted-foreground font-medium">{t("noLimits")}</p>
              <p className="text-muted-foreground text-sm">{t("noLimitsHint")}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("table.model")}</TableHead>
                    <TableHead>{t("table.rpm")}</TableHead>
                    <TableHead>{t("table.tpm")}</TableHead>
                    <TableHead>{t("table.dailyLimit")}</TableHead>
                    <TableHead>{t("table.status")}</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {limits.map((limit) => (
                    <TableRow key={limit.id}>
                      <TableCell className="font-medium">{limit.model}</TableCell>
                      <TableCell>{formatLimit(limit.rpm)}</TableCell>
                      <TableCell>{formatLimit(limit.tpm)}</TableCell>
                      <TableCell>
                        {limit.dailyRequests || limit.dailyTokens
                          ? `${formatLimit(limit.dailyRequests)} req / ${formatLimit(limit.dailyTokens)} tok`
                          : t("unlimited")}
                      </TableCell>
                      <TableCell>
                        <Badge variant={limit.enabled ? "default" : "secondary"}>
                          {limit.enabled
                            ? t("status.enabled", { ns: "common" })
                            : t("status.disabled", { ns: "common" })}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button
                            variant="ghost"
                            size="icon"
                            type="button"
                            aria-label={t("actions.edit", { ns: "common" })}
                            onClick={() => openEdit(limit)}
                          >
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            type="button"
                            aria-label={t("actions.delete", { ns: "common" })}
                            onClick={() => setDeleteConfirm(limit)}
                          >
                            <Trash2 className="text-destructive h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))}
                </TableBody>
              </Table>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

function LimitForm({
  form,
  onChange,
  onSubmit,
  isPending,
}: {
  form: LimitFormData
  onChange: (f: LimitFormData) => void
  onSubmit: () => void
  isPending: boolean
}) {
  const { t } = useTranslation("model-limits")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>{t("form.model")}</Label>
        <Input
          value={form.model}
          onChange={(e) => onChange({ ...form, model: e.target.value })}
          placeholder={t("form.modelPlaceholder")}
          required
        />
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label>{t("form.rpm")}</Label>
          <Input
            type="number"
            min="0"
            value={form.rpm}
            onChange={(e) => onChange({ ...form, rpm: e.target.value })}
          />
          <p className="text-muted-foreground text-xs">{t("form.rpmHint")}</p>
        </div>
        <div className="flex flex-col gap-2">
          <Label>{t("form.tpm")}</Label>
          <Input
            type="number"
            min="0"
            value={form.tpm}
            onChange={(e) => onChange({ ...form, tpm: e.target.value })}
          />
          <p className="text-muted-foreground text-xs">{t("form.tpmHint")}</p>
        </div>
      </div>
      <div className="grid grid-cols-2 gap-4">
        <div className="flex flex-col gap-2">
          <Label>{t("form.dailyRequests")}</Label>
          <Input
            type="number"
            min="0"
            value={form.dailyRequests}
            onChange={(e) => onChange({ ...form, dailyRequests: e.target.value })}
          />
          <p className="text-muted-foreground text-xs">{t("form.dailyRequestsHint")}</p>
        </div>
        <div className="flex flex-col gap-2">
          <Label>{t("form.dailyTokens")}</Label>
          <Input
            type="number"
            min="0"
            value={form.dailyTokens}
            onChange={(e) => onChange({ ...form, dailyTokens: e.target.value })}
          />
          <p className="text-muted-foreground text-xs">{t("form.dailyTokensHint")}</p>
        </div>
      </div>
      <div className="flex items-center gap-2">
        <Switch
          id="limit-enabled"
          checked={form.enabled}
          onCheckedChange={(checked) => onChange({ ...form, enabled: checked })}
        />
        <Label htmlFor="limit-enabled">{t("form.enabled")}</Label>
      </div>
      <Button type="submit" disabled={isPending}>
        {t("actions.save", { ns: "common" })}
      </Button>
    </form>
  )
}

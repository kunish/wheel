import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Pencil, Plus, RefreshCw, Search, Trash2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
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
import { Card, CardContent } from "@/components/ui/card"
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
  createModelPrice,
  deleteModelPrice,
  getLastPriceUpdateTime,
  listModelPrices,
  syncModelPrices,
  updateModelPrice,
} from "@/lib/api"

// ───────────── Types ─────────────

interface ModelPrice {
  id: number
  name: string
  inputPrice: number
  outputPrice: number
  source: string
  createdAt: string
  updatedAt: string
}

interface PriceFormData {
  name: string
  inputPrice: string
  outputPrice: string
}

const EMPTY_FORM: PriceFormData = {
  name: "",
  inputPrice: "",
  outputPrice: "",
}

// ───────────── Price Form ─────────────

function PriceForm({
  form,
  onChange,
  onSubmit,
  isPending,
  submitLabel,
  nameReadonly,
}: {
  form: PriceFormData
  onChange: (f: PriceFormData) => void
  onSubmit: () => void
  isPending: boolean
  submitLabel: string
  nameReadonly?: boolean
}) {
  const { t } = useTranslation("prices")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>{t("form.modelName")}</Label>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder="gpt-4o"
          required
          readOnly={nameReadonly}
          className={nameReadonly ? "bg-muted" : ""}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("form.inputPrice")}</Label>
        <Input
          type="number"
          step="0.000001"
          min="0"
          value={form.inputPrice}
          onChange={(e) => onChange({ ...form, inputPrice: e.target.value })}
          placeholder="0.000000"
          required
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("form.outputPrice")}</Label>
        <Input
          type="number"
          step="0.000001"
          min="0"
          value={form.outputPrice}
          onChange={(e) => onChange({ ...form, outputPrice: e.target.value })}
          placeholder="0.000000"
          required
        />
      </div>
      <Button type="submit" disabled={isPending}>
        {submitLabel}
      </Button>
    </form>
  )
}

// ───────────── Prices Page ─────────────

export default function PricesPage() {
  const { t } = useTranslation("prices")
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<PriceFormData>(EMPTY_FORM)
  const [editingPrice, setEditingPrice] = useState<ModelPrice | null>(null)
  const [editForm, setEditForm] = useState<PriceFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<ModelPrice | null>(null)

  const { data, isLoading } = useQuery({
    queryKey: ["model-prices"],
    queryFn: listModelPrices,
  })

  const { data: updateTimeData } = useQuery({
    queryKey: ["price-update-time"],
    queryFn: getLastPriceUpdateTime,
  })

  const createMutation = useMutation({
    mutationFn: (form: PriceFormData) =>
      createModelPrice({
        name: form.name,
        inputPrice: Number.parseFloat(form.inputPrice),
        outputPrice: Number.parseFloat(form.outputPrice),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      setCreateForm(EMPTY_FORM)
      setShowCreate(false)
      toast.success(t("toast.priceCreated"))
    },
    onError: () => toast.error(t("toast.createFailed")),
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: PriceFormData }) =>
      updateModelPrice({
        id,
        name: form.name,
        inputPrice: Number.parseFloat(form.inputPrice),
        outputPrice: Number.parseFloat(form.outputPrice),
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      setEditingPrice(null)
      toast.success(t("toast.priceUpdated"))
    },
    onError: () => toast.error(t("toast.updateFailed")),
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => deleteModelPrice({ name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      toast.success(t("toast.priceDeleted"))
    },
    onError: () => toast.error(t("toast.deleteFailed")),
  })

  const syncMutation = useMutation({
    mutationFn: syncModelPrices,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      queryClient.invalidateQueries({ queryKey: ["price-update-time"] })
      toast.success(t("toast.syncSuccess"))
    },
    onError: () => toast.error(t("toast.syncFailed")),
  })

  const models = useMemo(() => (data?.data?.models ?? []) as ModelPrice[], [data])
  const filtered = useMemo(
    () => models.filter((m) => m.name.toLowerCase().includes(search.toLowerCase())),
    [models, search],
  )

  function formatDateTime(dateStr: string | undefined): string | null {
    if (!dateStr) return null
    return new Date(dateStr).toLocaleString("en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
  }

  function openEdit(m: ModelPrice) {
    setEditForm({
      name: m.name,
      inputPrice: String(m.inputPrice),
      outputPrice: String(m.outputPrice),
    })
    setEditingPrice(m)
  }

  const lastSync = formatDateTime(updateTimeData?.data?.lastUpdateTime ?? undefined)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      {/* Toolbar */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-sm flex-1">
          <Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
          <label htmlFor="price-search" className="sr-only">
            {t("searchModels")}
          </label>
          <Input
            id="price-search"
            placeholder={t("searchPlaceholder")}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <div className="flex items-center gap-2">
          {lastSync && (
            <span className="text-muted-foreground text-xs">
              {t("lastSynced", { time: lastSync })}
            </span>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${syncMutation.isPending ? "animate-spin" : ""}`} />
            {t("syncPrices")}
          </Button>
          <Dialog
            open={showCreate}
            onOpenChange={(open) => {
              setShowCreate(open)
              if (!open) setCreateForm(EMPTY_FORM)
            }}
          >
            <DialogTrigger asChild>
              <Button size="sm">
                <Plus className="mr-2 h-4 w-4" /> {t("addPrice")}
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("addModelPrice")}</DialogTitle>
              </DialogHeader>
              <PriceForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => createMutation.mutate(createForm)}
                isPending={createMutation.isPending}
                submitLabel={t("actions.create", { ns: "common" })}
              />
            </DialogContent>
          </Dialog>
        </div>
      </div>

      {/* Edit Dialog */}
      <Dialog
        open={!!editingPrice}
        onOpenChange={(open) => {
          if (!open) setEditingPrice(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("editModelPrice")}</DialogTitle>
          </DialogHeader>
          <PriceForm
            form={editForm}
            onChange={setEditForm}
            onSubmit={() => {
              if (editingPrice) {
                updateMutation.mutate({ id: editingPrice.id, form: editForm })
              }
            }}
            isPending={updateMutation.isPending}
            submitLabel={t("actions.save", { ns: "common" })}
            nameReadonly
          />
        </DialogContent>
      </Dialog>

      {/* Delete Confirmation */}
      <AlertDialog open={!!deleteConfirm} onOpenChange={(open) => !open && setDeleteConfirm(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("deleteDialog.title", { name: deleteConfirm?.name })}
            </AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDialog.description")}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t("actions.cancel", { ns: "common" })}</AlertDialogCancel>
            <AlertDialogAction
              variant="destructive"
              onClick={() => {
                if (deleteConfirm) deleteMutation.mutate(deleteConfirm.name)
                setDeleteConfirm(null)
              }}
            >
              {t("actions.delete", { ns: "common" })}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>

      {/* Table */}
      {isLoading ? (
        <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
      ) : (
        <Card>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("table.modelName")}</TableHead>
                    <TableHead>{t("table.input")}</TableHead>
                    <TableHead>{t("table.output")}</TableHead>
                    <TableHead>{t("table.source")}</TableHead>
                    <TableHead>{t("table.updated")}</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {filtered.length === 0 ? (
                    <TableRow>
                      <TableCell colSpan={6} className="text-muted-foreground text-center">
                        {t("table.noModels")}
                      </TableCell>
                    </TableRow>
                  ) : (
                    <AnimatePresence initial={false}>
                      {filtered.map((m) => (
                        <motion.tr
                          key={m.id}
                          initial={{ opacity: 0, y: -10 }}
                          animate={{ opacity: 1, y: 0 }}
                          exit={{ opacity: 0, y: -10 }}
                          transition={{ duration: 0.2 }}
                          className="border-b"
                        >
                          <TableCell className="font-medium">
                            <ModelBadge modelId={m.name} />
                          </TableCell>
                          <TableCell>{m.inputPrice.toFixed(6)}</TableCell>
                          <TableCell>{m.outputPrice.toFixed(6)}</TableCell>
                          <TableCell>
                            <Badge variant={m.source === "sync" ? "secondary" : "default"}>
                              {m.source}
                            </Badge>
                          </TableCell>
                          <TableCell>{formatDateTime(m.updatedAt) ?? "-"}</TableCell>
                          <TableCell>
                            <div className="flex gap-1">
                              <Button
                                variant="ghost"
                                size="icon"
                                aria-label={t("aria.editPrice", { name: m.name })}
                                onClick={() => openEdit(m)}
                              >
                                <Pencil className="h-4 w-4" />
                              </Button>
                              <Button
                                variant="ghost"
                                size="icon"
                                aria-label={t("aria.deletePrice", { name: m.name })}
                                onClick={() => setDeleteConfirm(m)}
                              >
                                <Trash2 className="text-destructive h-4 w-4" />
                              </Button>
                            </div>
                          </TableCell>
                        </motion.tr>
                      ))}
                    </AnimatePresence>
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

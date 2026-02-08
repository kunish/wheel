"use client"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Pencil, Plus, RefreshCw, Search, Trash2 } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"
import { ModelBadge } from "@/components/model-badge"
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
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>Model Name</Label>
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
        <Label>Input Price ($ / M tokens)</Label>
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
        <Label>Output Price ($ / M tokens)</Label>
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
  const queryClient = useQueryClient()
  const [search, setSearch] = useState("")
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<PriceFormData>(EMPTY_FORM)
  const [editingPrice, setEditingPrice] = useState<ModelPrice | null>(null)
  const [editForm, setEditForm] = useState<PriceFormData>(EMPTY_FORM)

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
      toast.success("Price created")
    },
    onError: () => toast.error("Failed to create price"),
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
      toast.success("Price updated")
    },
    onError: () => toast.error("Failed to update price"),
  })

  const deleteMutation = useMutation({
    mutationFn: (name: string) => deleteModelPrice({ name }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      toast.success("Price deleted")
    },
    onError: () => toast.error("Failed to delete price"),
  })

  const syncMutation = useMutation({
    mutationFn: syncModelPrices,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["model-prices"] })
      queryClient.invalidateQueries({ queryKey: ["price-update-time"] })
      toast.success("Prices synced successfully")
    },
    onError: () => toast.error("Failed to sync prices"),
  })

  const models = (data?.data?.models ?? []) as ModelPrice[]
  const filtered = models.filter((m) => m.name.toLowerCase().includes(search.toLowerCase()))

  function formatDate(dateStr: string) {
    if (!dateStr) return "-"
    const d = new Date(dateStr)
    return d.toLocaleString("en-US", {
      year: "numeric",
      month: "2-digit",
      day: "2-digit",
      hour: "2-digit",
      minute: "2-digit",
    })
  }

  function formatLastSync(dateStr: string | undefined) {
    if (!dateStr) return null
    const d = new Date(dateStr)
    return d.toLocaleString("en-US", {
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

  const lastSync = formatLastSync(updateTimeData?.data?.lastUpdateTime ?? undefined)

  return (
    <div className="flex flex-col gap-6">
      <div>
        <h2 className="text-2xl font-bold tracking-tight">Prices</h2>
        <p className="text-muted-foreground text-sm">Manage model pricing for cost calculation</p>
      </div>

      {/* Toolbar */}
      <div className="flex flex-col gap-4 sm:flex-row sm:items-center sm:justify-between">
        <div className="relative max-w-sm flex-1">
          <Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
          <Input
            placeholder="Search models..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pl-9"
          />
        </div>
        <div className="flex items-center gap-2">
          {lastSync && (
            <span className="text-muted-foreground text-xs">Last synced: {lastSync}</span>
          )}
          <Button
            variant="outline"
            size="sm"
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
          >
            <RefreshCw className={`mr-2 h-4 w-4 ${syncMutation.isPending ? "animate-spin" : ""}`} />
            Sync Prices
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
                <Plus className="mr-2 h-4 w-4" /> Add Price
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>Add Model Price</DialogTitle>
              </DialogHeader>
              <PriceForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => createMutation.mutate(createForm)}
                isPending={createMutation.isPending}
                submitLabel="Create"
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
            <DialogTitle>Edit Model Price</DialogTitle>
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
            submitLabel="Save"
            nameReadonly
          />
        </DialogContent>
      </Dialog>

      {/* Table */}
      {isLoading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : (
        <Card>
          <CardContent className="p-0">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Model Name</TableHead>
                  <TableHead>Input ($/M tokens)</TableHead>
                  <TableHead>Output ($/M tokens)</TableHead>
                  <TableHead>Source</TableHead>
                  <TableHead>Updated</TableHead>
                  <TableHead className="w-24" />
                </TableRow>
              </TableHeader>
              <TableBody>
                {filtered.length === 0 ? (
                  <TableRow>
                    <TableCell colSpan={6} className="text-muted-foreground text-center">
                      No models found.
                    </TableCell>
                  </TableRow>
                ) : (
                  filtered.map((m) => (
                    <TableRow key={m.id}>
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
                      <TableCell>{formatDate(m.updatedAt)}</TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button variant="ghost" size="icon" onClick={() => openEdit(m)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => {
                              if (confirm(`Delete price for "${m.name}"?`)) {
                                deleteMutation.mutate(m.name)
                              }
                            }}
                          >
                            <Trash2 className="text-destructive h-4 w-4" />
                          </Button>
                        </div>
                      </TableCell>
                    </TableRow>
                  ))
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>
      )}
    </div>
  )
}

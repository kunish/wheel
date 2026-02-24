import type { DragEndEvent } from "@dnd-kit/core"
import { DndContext, PointerSensor, useSensor, useSensors } from "@dnd-kit/core"
import {
  arrayMove,
  SortableContext,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import { GripVertical, Plus, Search, X } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { ModelCard } from "@/components/model-card"
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
import { Switch } from "@/components/ui/switch"
import { cn } from "@/lib/utils"
import ChannelModelPickerDialog from "./channel-model-picker-dialog"
import ModelPickerDialog from "./model-picker-dialog"

export interface GroupItemForm {
  channelId: number
  modelName: string
  priority: number
  weight: number
  enabled: boolean
}

export interface GroupFormData {
  id?: number
  name: string
  mode: number
  firstTokenTimeOut: number
  sessionKeepTime: number
  items: GroupItemForm[]
}

export const EMPTY_GROUP_FORM: GroupFormData = {
  name: "",
  mode: 1,
  firstTokenTimeOut: 0,
  sessionKeepTime: 0,
  items: [],
}

// ─── Sortable Dialog Item ─────────────────────

function SortableDialogItem({
  id,
  item,
  mode,
  channelOptions,
  onEdit,
  onUpdate,
  onRemove,
}: {
  id: string
  item: GroupItemForm
  mode: number
  channelOptions: { id: number; name: string; model: string[]; fetchedModel: string[] }[]
  onEdit: () => void
  onUpdate: (patch: Partial<GroupItemForm>) => void
  onRemove: () => void
}) {
  const { t } = useTranslation("model")
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id,
  })

  const style = {
    transform: CSS.Translate.toString(transform),
    transition,
  }

  return (
    <div
      ref={setNodeRef}
      style={style}
      className={cn(
        "flex min-w-0 items-center gap-2 rounded-md border p-2",
        isDragging && "border-dashed opacity-30",
        !item.enabled && "opacity-50",
      )}
    >
      <button
        {...attributes}
        {...listeners}
        className="text-muted-foreground hover:text-foreground hover:bg-accent shrink-0 cursor-grab touch-none rounded p-1"
      >
        <GripVertical className="h-4 w-4" />
      </button>
      <button type="button" onClick={onEdit} className="min-w-0 flex-1 cursor-pointer text-left">
        <ModelCard modelId={item.modelName || t("groupDialog.emptyModel")} />
      </button>
      <Select
        value={item.channelId ? String(item.channelId) : ""}
        onValueChange={(v) => onUpdate({ channelId: Number(v) })}
      >
        <SelectTrigger className="w-24 shrink-0">
          <SelectValue placeholder={t("groupDialog.channelPlaceholder")} />
        </SelectTrigger>
        <SelectContent>
          {channelOptions.map((ch) => (
            <SelectItem key={ch.id} value={String(ch.id)}>
              {ch.name}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      {mode === 3 && (
        <Input
          className="w-20"
          type="number"
          placeholder={t("groupDialog.priority")}
          value={item.priority}
          onChange={(e) => onUpdate({ priority: Number(e.target.value) })}
        />
      )}
      {mode === 4 && (
        <Input
          className="w-20"
          type="number"
          placeholder={t("groupDialog.weight")}
          value={item.weight}
          onChange={(e) => onUpdate({ weight: Number(e.target.value) })}
        />
      )}
      <Switch
        checked={item.enabled}
        onCheckedChange={(checked) => onUpdate({ enabled: checked })}
        className="shrink-0"
      />
      <Button variant="ghost" size="icon" className="shrink-0" onClick={onRemove}>
        <X className="h-4 w-4" />
      </Button>
    </div>
  )
}

// ─── Group Dialog ──────────────────────────────

interface GroupDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: GroupFormData
  setForm: (f: GroupFormData) => void
  channelOptions: { id: number; name: string; model: string[]; fetchedModel: string[] }[]
  onSave: () => void
  isPending: boolean
}

export default function GroupDialog({
  open,
  onOpenChange,
  form,
  setForm,
  channelOptions,
  onSave,
  isPending,
}: GroupDialogProps) {
  const { t } = useTranslation("model")
  const [modelPickerOpen, setModelPickerOpen] = useState(false)
  // null = closed, -1 = adding new item, >= 0 = editing item at index
  const [editingItemIndex, setEditingItemIndex] = useState<number | null>(null)

  const modeLabels = useMemo(
    () => ({
      1: t("modeLabels.1"),
      2: t("modeLabels.2"),
      3: t("modeLabels.3"),
      4: t("modeLabels.4"),
    }),
    [t],
  )

  const dialogSensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: 5 } }),
  )
  const dialogItemIds = useMemo(() => form.items.map((_, i) => `dialog-item-${i}`), [form.items])

  function handleDialogDragEnd(event: DragEndEvent) {
    const { active, over } = event
    if (!over || active.id === over.id) return

    const oldIndex = Number(String(active.id).replace("dialog-item-", ""))
    const newIndex = Number(String(over.id).replace("dialog-item-", ""))
    if (oldIndex === newIndex) return

    setForm({ ...form, items: arrayMove(form.items, oldIndex, newIndex) })
  }

  // Build channel→models mapping for the item model picker
  const channelModels = useMemo(() => {
    return channelOptions
      .filter((ch) => ch.model && ch.model.length > 0)
      .map((ch) => ({
        channelId: ch.id,
        channelName: ch.name,
        models: ch.model.filter(Boolean).sort(),
        fetchedModels: ch.fetchedModel ?? [],
      }))
  }, [channelOptions])

  function updateItem(index: number, patch: Partial<GroupItemForm>) {
    const items = [...form.items]
    items[index] = { ...items[index], ...patch }
    setForm({ ...form, items })
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] w-full max-w-2xl max-w-[95vw] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {form.id ? t("groupDialog.editTitle") : t("groupDialog.createTitle")}
          </DialogTitle>
        </DialogHeader>
        <div className="flex min-w-0 flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label>{t("groupDialog.name")}</Label>
            <div className="relative">
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder="e.g. claude-opus-4-6"
                className="pr-28"
              />
              <button
                type="button"
                className="text-muted-foreground hover:text-accent-foreground hover:bg-accent absolute top-1/2 right-1.5 inline-flex -translate-y-1/2 items-center rounded-md px-2 py-1 text-xs transition-colors"
                onClick={() => setModelPickerOpen(true)}
              >
                <Search className="mr-1 h-3 w-3" />
                models.dev
              </button>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("groupDialog.loadBalancingMode")}</Label>
            <Select
              value={String(form.mode)}
              onValueChange={(v) => setForm({ ...form, mode: Number(v) })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(modeLabels).map(([val, label]) => (
                  <SelectItem key={val} value={val}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("groupDialog.firstTokenTimeout")}</Label>
            <Input
              type="number"
              value={form.firstTokenTimeOut}
              onChange={(e) =>
                setForm({
                  ...form,
                  firstTokenTimeOut: Number(e.target.value),
                })
              }
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("groupDialog.sessionKeepTime")}</Label>
            <Input
              type="number"
              value={form.sessionKeepTime}
              onChange={(e) =>
                setForm({
                  ...form,
                  sessionKeepTime: Number(e.target.value),
                })
              }
              placeholder={t("groupDialog.sessionKeepTimePlaceholder")}
            />
          </div>

          {/* Group Items */}
          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>{t("groupDialog.models")}</Label>
              <Button variant="outline" size="sm" onClick={() => setEditingItemIndex(-1)}>
                <Plus className="mr-1 h-3 w-3" /> {t("actions.add", { ns: "common" })}
              </Button>
            </div>
            {form.items.length === 0 && (
              <p className="text-muted-foreground text-sm">{t("groupDialog.emptyItems")}</p>
            )}
            {/* eslint-disable react/no-array-index-key -- items may have duplicate channelId+modelName */}
            <DndContext sensors={dialogSensors} onDragEnd={handleDialogDragEnd}>
              <SortableContext items={dialogItemIds} strategy={verticalListSortingStrategy}>
                {form.items.map((item, i) => (
                  <SortableDialogItem
                    key={`${item.channelId}-${item.modelName}-${i}`}
                    id={`dialog-item-${i}`}
                    item={item}
                    mode={form.mode}
                    channelOptions={channelOptions}
                    onEdit={() => setEditingItemIndex(i)}
                    onUpdate={(patch) => updateItem(i, patch)}
                    onRemove={() =>
                      setForm({
                        ...form,
                        items: form.items.filter((_, j) => j !== i),
                      })
                    }
                  />
                ))}
              </SortableContext>
            </DndContext>
            {/* eslint-enable react/no-array-index-key */}
          </div>

          <Button className="mt-2" onClick={onSave} disabled={isPending || !form.name}>
            {isPending ? t("groupDialog.saving") : t("actions.save", { ns: "common" })}
          </Button>
        </div>
      </DialogContent>

      <ModelPickerDialog
        open={modelPickerOpen}
        onOpenChange={setModelPickerOpen}
        onSelect={(modelId: string) => {
          setForm({ ...form, name: modelId })
          setModelPickerOpen(false)
        }}
      />

      <ChannelModelPickerDialog
        open={editingItemIndex !== null}
        onOpenChange={(open) => {
          if (!open) setEditingItemIndex(null)
        }}
        channelModels={channelModels}
        onSelect={(channelId: number, modelId: string) => {
          if (editingItemIndex !== null && editingItemIndex >= 0) {
            // Editing existing item — replace model and channel
            updateItem(editingItemIndex, { modelName: modelId, channelId })
          } else {
            // Adding new item
            setForm({
              ...form,
              items: [
                ...form.items,
                { channelId, modelName: modelId, priority: 0, weight: 1, enabled: true },
              ],
            })
          }
          setEditingItemIndex(null)
        }}
      />
    </Dialog>
  )
}

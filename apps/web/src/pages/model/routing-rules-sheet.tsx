import type {
  RoutingActionItem,
  RoutingConditionItem,
  RoutingRule,
  RoutingRuleInput,
} from "@/lib/api-client"
import { useQuery } from "@tanstack/react-query"
import { Pencil, Plus, Trash2, X } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
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
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listRoutingRules } from "@/lib/api-client"
import { useRoutingRuleMutations } from "./use-routing-rule-mutations"

const EMPTY_CONDITION: RoutingConditionItem = { field: "model", operator: "eq", value: "" }
const EMPTY_ACTION: RoutingActionItem = { type: "reject", statusCode: 403, message: "" }
const EMPTY_FORM: RoutingRuleInput = {
  name: "",
  priority: 0,
  enabled: true,
  conditions: [{ ...EMPTY_CONDITION }],
  action: { ...EMPTY_ACTION },
}

const FIELD_OPTIONS = ["model", "apikey_name", "request_type", "header:", "body:"] as const
const OPERATOR_OPTIONS = ["eq", "neq", "contains", "prefix", "suffix", "regex", "in"] as const
const ACTION_TYPES = ["reject", "route", "rewrite"] as const

export default function RoutingRulesSheet({
  open,
  onOpenChange,
  groupNames,
  modelNames,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  groupNames: string[]
  modelNames: string[]
}) {
  const { t } = useTranslation("model")

  const { data, isLoading } = useQuery({
    queryKey: ["routing-rules"],
    queryFn: listRoutingRules,
    enabled: open,
  })
  const rules = data?.data?.rules ?? []

  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<RoutingRuleInput>({ ...EMPTY_FORM })
  const [editingId, setEditingId] = useState<number | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<RoutingRule | null>(null)

  const { createMut, updateMut, deleteMut, toggleMut } = useRoutingRuleMutations({
    onSaveSuccess: () => setDialogOpen(false),
  })

  function openCreate() {
    setEditingId(null)
    setForm({ ...EMPTY_FORM, conditions: [{ ...EMPTY_CONDITION }], action: { ...EMPTY_ACTION } })
    setDialogOpen(true)
  }

  function openEdit(rule: RoutingRule) {
    setEditingId(rule.id)
    setForm({
      name: rule.name,
      priority: rule.priority,
      enabled: rule.enabled,
      conditions:
        rule.conditions.length > 0
          ? rule.conditions.map((c) => ({ ...c }))
          : [{ ...EMPTY_CONDITION }],
      action: { ...rule.action },
    })
    setDialogOpen(true)
  }

  function handleSave() {
    // Validate required fields
    if (!form.name.trim()) return
    if (form.action.type === "route" && !form.action.groupName) return
    if (form.action.type === "rewrite" && !form.action.modelName) return
    if (form.conditions.some((c) => !c.value.trim())) return

    if (editingId !== null) {
      updateMut.mutate({ ...form, id: editingId })
    } else {
      createMut.mutate(form)
    }
  }

  const isPending = createMut.isPending || updateMut.isPending

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="right" className="w-full overflow-y-auto sm:max-w-2xl">
        <SheetHeader>
          <SheetTitle>{t("routingRules.title")}</SheetTitle>
          <SheetDescription>{t("routingRules.description")}</SheetDescription>
        </SheetHeader>

        <div className="flex flex-col gap-4 px-4 pb-4">
          <div className="flex justify-end">
            <Button size="sm" onClick={openCreate}>
              <Plus className="mr-1 h-4 w-4" />
              {t("routingRules.createRule")}
            </Button>
          </div>

          {isLoading ? (
            <p className="text-muted-foreground text-sm">
              {t("actions.loading", { ns: "common" })}
            </p>
          ) : rules.length === 0 ? (
            <p className="text-muted-foreground text-sm">{t("routingRules.noRules")}</p>
          ) : (
            <RulesTable
              rules={rules}
              t={t}
              onEdit={openEdit}
              onDelete={setDeleteTarget}
              onToggle={(id, enabled) => toggleMut.mutate({ id, enabled })}
            />
          )}
        </div>

        <RuleDialog
          open={dialogOpen}
          onOpenChange={setDialogOpen}
          form={form}
          setForm={setForm}
          onSave={handleSave}
          isPending={isPending}
          isEdit={editingId !== null}
          t={t}
          groupNames={groupNames}
          modelNames={modelNames}
        />

        <AlertDialog open={deleteTarget !== null} onOpenChange={(o) => !o && setDeleteTarget(null)}>
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                {t("routingRules.deleteTitle", { name: deleteTarget?.name })}
              </AlertDialogTitle>
              <AlertDialogDescription>{t("routingRules.deleteDescription")}</AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>{t("actions.cancel", { ns: "common" })}</AlertDialogCancel>
              <AlertDialogAction
                onClick={() => deleteTarget && deleteMut.mutate(deleteTarget.id)}
                className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
              >
                {t("actions.delete", { ns: "common" })}
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>
      </SheetContent>
    </Sheet>
  )
}

// ── Rules Table ──

function RulesTable({
  rules,
  t,
  onEdit,
  onDelete,
  onToggle,
}: {
  rules: RoutingRule[]
  t: (key: string, opts?: Record<string, unknown>) => string
  onEdit: (rule: RoutingRule) => void
  onDelete: (rule: RoutingRule) => void
  onToggle: (id: number, enabled: boolean) => void
}) {
  function formatConditions(conditions: RoutingConditionItem[]) {
    return conditions
      .map((c) => {
        const fieldLabel = c.field.includes(":") ? c.field : t(`routingRules.fields.${c.field}`)
        const opLabel = t(`routingRules.operators.${c.operator}`)
        return `${fieldLabel} ${opLabel} "${c.value}"`
      })
      .join(" AND ")
  }

  function formatAction(action: RoutingActionItem) {
    const label = t(`routingRules.actions.${action.type}`)
    switch (action.type) {
      case "reject":
        return `${label} (${action.statusCode ?? 403})`
      case "route":
        return `${label}: ${action.groupName}`
      case "rewrite":
        return `${label}: ${action.modelName}`
      default:
        return label
    }
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="w-[120px]">{t("routingRules.table.name")}</TableHead>
          <TableHead className="w-[60px]">{t("routingRules.table.priority")}</TableHead>
          <TableHead>{t("routingRules.table.conditions")}</TableHead>
          <TableHead className="w-[150px]">{t("routingRules.table.action")}</TableHead>
          <TableHead className="w-[50px]">{t("routingRules.table.status")}</TableHead>
          <TableHead className="w-[70px]" />
        </TableRow>
      </TableHeader>
      <TableBody>
        {rules.map((rule) => (
          <TableRow key={rule.id}>
            <TableCell className="font-medium">{rule.name}</TableCell>
            <TableCell>{rule.priority}</TableCell>
            <TableCell className="max-w-[200px] truncate text-xs">
              {formatConditions(rule.conditions)}
            </TableCell>
            <TableCell>
              <Badge variant={rule.action.type === "reject" ? "destructive" : "secondary"}>
                {formatAction(rule.action)}
              </Badge>
            </TableCell>
            <TableCell>
              <Switch
                checked={rule.enabled}
                onCheckedChange={(v) => onToggle(rule.id, v)}
                aria-label={rule.name}
              />
            </TableCell>
            <TableCell>
              <div className="flex gap-1">
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => onEdit(rule)}
                >
                  <Pencil className="h-3.5 w-3.5" />
                </Button>
                <Button
                  variant="ghost"
                  size="icon"
                  className="h-7 w-7"
                  onClick={() => onDelete(rule)}
                >
                  <Trash2 className="h-3.5 w-3.5" />
                </Button>
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// ── Rule Dialog ──

function RuleDialog({
  open,
  onOpenChange,
  form,
  setForm,
  onSave,
  isPending,
  isEdit,
  t,
  groupNames,
  modelNames,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: RoutingRuleInput
  setForm: (f: RoutingRuleInput) => void
  onSave: () => void
  isPending: boolean
  isEdit: boolean
  t: (key: string, opts?: Record<string, unknown>) => string
  groupNames: string[]
  modelNames: string[]
}) {
  function updateCondition(idx: number, patch: Partial<RoutingConditionItem>) {
    const next = [...form.conditions]
    next[idx] = { ...next[idx], ...patch }
    setForm({ ...form, conditions: next })
  }

  function removeCondition(idx: number) {
    const next = form.conditions.filter((_, i) => i !== idx)
    setForm({ ...form, conditions: next.length > 0 ? next : [{ ...EMPTY_CONDITION }] })
  }

  function addCondition() {
    setForm({ ...form, conditions: [...form.conditions, { ...EMPTY_CONDITION }] })
  }

  function updateAction(patch: Partial<RoutingActionItem>) {
    // If switching action type, replace entirely to clear residual fields
    if (patch.type && patch.type !== form.action.type) {
      setForm({ ...form, action: patch as RoutingActionItem })
    } else {
      setForm({ ...form, action: { ...form.action, ...patch } })
    }
  }

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] w-full max-w-2xl overflow-y-auto">
        <DialogHeader>
          <DialogTitle>
            {isEdit ? t("routingRules.editRule") : t("routingRules.createRule")}
          </DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="grid grid-cols-[1fr_100px_auto] gap-3">
            <div className="flex flex-col gap-1.5">
              <Label>{t("routingRules.form.name")}</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder={t("routingRules.form.namePlaceholder")}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label>{t("routingRules.form.priority")}</Label>
              <Input
                type="number"
                min={0}
                value={form.priority}
                onChange={(e) => setForm({ ...form, priority: Number(e.target.value) })}
              />
            </div>
            <div className="flex flex-col items-center gap-1.5">
              <Label>{t("routingRules.form.enabled")}</Label>
              <Switch
                checked={form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
                className="mt-1"
              />
            </div>
          </div>
          <p className="text-muted-foreground -mt-2 text-xs">
            {t("routingRules.form.priorityHint")}
          </p>

          <ConditionEditor
            conditions={form.conditions}
            updateCondition={updateCondition}
            removeCondition={removeCondition}
            addCondition={addCondition}
            t={t}
          />

          <ActionEditor
            action={form.action}
            updateAction={updateAction}
            t={t}
            groupNames={groupNames}
            modelNames={modelNames}
          />

          <Button type="button" onClick={onSave} disabled={isPending || !form.name}>
            {isPending
              ? t("actions.saving", { ns: "common" })
              : t("actions.save", { ns: "common" })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── Condition Editor ──

function ConditionEditor({
  conditions,
  updateCondition,
  removeCondition,
  addCondition,
  t,
}: {
  conditions: RoutingConditionItem[]
  updateCondition: (idx: number, patch: Partial<RoutingConditionItem>) => void
  removeCondition: (idx: number) => void
  addCondition: () => void
  t: (key: string) => string
}) {
  function getFieldBase(field: string) {
    if (field.startsWith("header:")) return "header:"
    if (field.startsWith("body:")) return "body:"
    return field
  }

  function getFieldSuffix(field: string) {
    if (field.startsWith("header:")) return field.slice(7)
    if (field.startsWith("body:")) return field.slice(5)
    return ""
  }

  return (
    <div className="flex flex-col gap-2">
      <Label>{t("routingRules.form.conditions")}</Label>
      {conditions.map((cond, idx) => (
        <div key={idx} className="flex items-center gap-2">
          <Select
            value={getFieldBase(cond.field)}
            onValueChange={(v) => {
              // For prefix fields (header:, body:), keep the colon; for others, use as-is
              updateCondition(idx, { field: v })
            }}
          >
            <SelectTrigger className="w-[130px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {FIELD_OPTIONS.map((f) => (
                <SelectItem key={f} value={f}>
                  {f.endsWith(":")
                    ? t(`routingRules.fields.${f.slice(0, -1)}`)
                    : t(`routingRules.fields.${f}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          {(cond.field.startsWith("header:") || cond.field.startsWith("body:")) && (
            <Input
              className="w-[100px]"
              placeholder={cond.field.startsWith("header:") ? "X-Custom" : "path.to.field"}
              value={getFieldSuffix(cond.field)}
              onChange={(e) =>
                updateCondition(idx, { field: getFieldBase(cond.field) + e.target.value })
              }
            />
          )}

          <Select
            value={cond.operator}
            onValueChange={(v) => updateCondition(idx, { operator: v })}
          >
            <SelectTrigger className="w-[120px]">
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {OPERATOR_OPTIONS.map((op) => (
                <SelectItem key={op} value={op}>
                  {t(`routingRules.operators.${op}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>

          <Input
            className="flex-1"
            placeholder="value"
            value={cond.value}
            onChange={(e) => updateCondition(idx, { value: e.target.value })}
          />

          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8 shrink-0"
            onClick={() => removeCondition(idx)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" className="w-fit" onClick={addCondition}>
        <Plus className="mr-1 h-3.5 w-3.5" />
        {t("routingRules.form.addCondition")}
      </Button>
    </div>
  )
}

// ── Action Editor ──

function ActionEditor({
  action,
  updateAction,
  t,
  groupNames,
  modelNames,
}: {
  action: RoutingActionItem
  updateAction: (patch: Partial<RoutingActionItem>) => void
  t: (key: string) => string
  groupNames: string[]
  modelNames: string[]
}) {
  return (
    <div className="flex flex-col gap-2">
      <Label>{t("routingRules.form.action")}</Label>
      <div className="flex flex-col gap-3 rounded-md border p-3">
        <div className="flex flex-col gap-1.5">
          <Label className="text-xs">{t("routingRules.form.actionType")}</Label>
          <Select
            value={action.type}
            onValueChange={(v) => {
              // Clear residual fields from previous action type
              const newAction: RoutingActionItem = { type: v as RoutingActionItem["type"] }
              if (v === "reject") {
                newAction.statusCode = 403
                newAction.message = ""
              }
              updateAction(newAction)
            }}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              {ACTION_TYPES.map((at) => (
                <SelectItem key={at} value={at}>
                  {t(`routingRules.actions.${at}`)}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>

        {action.type === "reject" && (
          <>
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">{t("routingRules.form.statusCode")}</Label>
              <Input
                type="number"
                min={400}
                max={599}
                value={action.statusCode ?? 403}
                onChange={(e) => updateAction({ statusCode: Number(e.target.value) })}
              />
            </div>
            <div className="flex flex-col gap-1.5">
              <Label className="text-xs">{t("routingRules.form.message")}</Label>
              <Input
                value={action.message ?? ""}
                onChange={(e) => updateAction({ message: e.target.value })}
                placeholder={t("routingRules.form.messagePlaceholder")}
              />
            </div>
          </>
        )}

        {action.type === "route" && (
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">{t("routingRules.form.groupName")}</Label>
            {groupNames.length > 0 ? (
              <Select
                value={action.groupName ?? ""}
                onValueChange={(v) => updateAction({ groupName: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("routingRules.form.groupNamePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {groupNames.map((name) => (
                    <SelectItem key={name} value={name}>
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                value={action.groupName ?? ""}
                onChange={(e) => updateAction({ groupName: e.target.value })}
                placeholder={t("routingRules.form.groupNamePlaceholder")}
              />
            )}
          </div>
        )}

        {action.type === "rewrite" && (
          <div className="flex flex-col gap-1.5">
            <Label className="text-xs">{t("routingRules.form.modelName")}</Label>
            {modelNames.length > 0 ? (
              <Select
                value={action.modelName ?? ""}
                onValueChange={(v) => updateAction({ modelName: v })}
              >
                <SelectTrigger>
                  <SelectValue placeholder={t("routingRules.form.modelNamePlaceholder")} />
                </SelectTrigger>
                <SelectContent>
                  {modelNames.map((name) => (
                    <SelectItem key={name} value={name}>
                      {name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            ) : (
              <Input
                value={action.modelName ?? ""}
                onChange={(e) => updateAction({ modelName: e.target.value })}
                placeholder={t("routingRules.form.modelNamePlaceholder")}
              />
            )}
          </div>
        )}
      </div>
    </div>
  )
}

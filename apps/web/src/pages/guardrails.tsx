import { Pencil, Plus, Shield, Trash2 } from "lucide-react"
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
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

// Local state management (no backend API yet — this is a UI-ready scaffold)

interface GuardrailRule {
  id: string
  name: string
  type: "keyword" | "regex" | "length" | "pii"
  target: "input" | "output" | "both"
  action: "block" | "warn" | "redact"
  pattern: string
  maxLength?: number
  enabled: boolean
}

interface RuleFormData {
  name: string
  type: "keyword" | "regex" | "length" | "pii"
  target: "input" | "output" | "both"
  action: "block" | "warn" | "redact"
  pattern: string
  maxLength: string
  enabled: boolean
}

const EMPTY_FORM: RuleFormData = {
  name: "",
  type: "keyword",
  target: "both",
  action: "block",
  pattern: "",
  maxLength: "",
  enabled: true,
}

export default function GuardrailsPage() {
  const { t } = useTranslation("guardrails")
  const [rules, setRules] = useState<GuardrailRule[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<RuleFormData>(EMPTY_FORM)
  const [editingRule, setEditingRule] = useState<GuardrailRule | null>(null)
  const [editForm, setEditForm] = useState<RuleFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<GuardrailRule | null>(null)

  function handleCreate(form: RuleFormData) {
    const rule: GuardrailRule = {
      id: crypto.randomUUID(),
      name: form.name,
      type: form.type,
      target: form.target,
      action: form.action,
      pattern: form.pattern,
      maxLength: form.maxLength ? Number.parseInt(form.maxLength, 10) : undefined,
      enabled: form.enabled,
    }
    setRules((prev) => [...prev, rule])
    setCreateForm(EMPTY_FORM)
    setShowCreate(false)
    toast.success(t("ruleSaved"))
  }

  function handleUpdate(id: string, form: RuleFormData) {
    setRules((prev) =>
      prev.map((r) =>
        r.id === id
          ? {
              ...r,
              name: form.name,
              type: form.type,
              target: form.target,
              action: form.action,
              pattern: form.pattern,
              maxLength: form.maxLength ? Number.parseInt(form.maxLength, 10) : undefined,
              enabled: form.enabled,
            }
          : r,
      ),
    )
    setEditingRule(null)
    toast.success(t("ruleSaved"))
  }

  function handleDelete(id: string) {
    setRules((prev) => prev.filter((r) => r.id !== id))
    setDeleteConfirm(null)
    toast.success(t("ruleDeleted"))
  }

  function openEdit(rule: GuardrailRule) {
    setEditForm({
      name: rule.name,
      type: rule.type,
      target: rule.target,
      action: rule.action,
      pattern: rule.pattern,
      maxLength: rule.maxLength ? String(rule.maxLength) : "",
      enabled: rule.enabled,
    })
    setEditingRule(rule)
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

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
              <Button size="sm">
                <Plus className="mr-2 h-4 w-4" /> {t("addRule")}
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("addRule")}</DialogTitle>
              </DialogHeader>
              <RuleForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => handleCreate(createForm)}
              />
            </DialogContent>
          </Dialog>
        </CardHeader>
        <CardContent>
          {/* Edit dialog */}
          <Dialog
            open={!!editingRule}
            onOpenChange={(open) => {
              if (!open) setEditingRule(null)
            }}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("editRule")}</DialogTitle>
              </DialogHeader>
              <RuleForm
                form={editForm}
                onChange={setEditForm}
                onSubmit={() => editingRule && handleUpdate(editingRule.id, editForm)}
              />
            </DialogContent>
          </Dialog>

          {/* Delete dialog */}
          <ConfirmDeleteDialog
            open={!!deleteConfirm}
            onOpenChange={(open) => !open && setDeleteConfirm(null)}
            title={t("deleteTitle", { name: deleteConfirm?.name })}
            description={t("deleteDescription")}
            cancelLabel={t("actions.cancel", { ns: "common" })}
            confirmLabel={t("actions.delete", { ns: "common" })}
            onConfirm={() => deleteConfirm && handleDelete(deleteConfirm.id)}
          />

          {rules.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-16">
              <Shield className="text-muted-foreground h-10 w-10" />
              <p className="text-muted-foreground font-medium">{t("noRules")}</p>
              <p className="text-muted-foreground text-sm">{t("noRulesHint")}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("table.name")}</TableHead>
                    <TableHead>{t("table.type")}</TableHead>
                    <TableHead>{t("table.target")}</TableHead>
                    <TableHead>{t("table.action")}</TableHead>
                    <TableHead>{t("table.status")}</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {rules.map((rule) => (
                    <TableRow key={rule.id}>
                      <TableCell className="font-medium">{rule.name}</TableCell>
                      <TableCell>
                        <Badge variant="outline">{t(`type.${rule.type}`)}</Badge>
                      </TableCell>
                      <TableCell>{t(`target.${rule.target}`)}</TableCell>
                      <TableCell>
                        <Badge variant={rule.action === "block" ? "destructive" : "secondary"}>
                          {t(`action.${rule.action}`)}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <Badge variant={rule.enabled ? "default" : "secondary"}>
                          {rule.enabled
                            ? t("status.enabled", { ns: "common" })
                            : t("status.disabled", { ns: "common" })}
                        </Badge>
                      </TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button variant="ghost" size="icon" onClick={() => openEdit(rule)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button
                            variant="ghost"
                            size="icon"
                            onClick={() => setDeleteConfirm(rule)}
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

function RuleForm({
  form,
  onChange,
  onSubmit,
}: {
  form: RuleFormData
  onChange: (f: RuleFormData) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation("guardrails")
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>{t("form.name")}</Label>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder={t("form.namePlaceholder")}
          required
        />
      </div>
      <div className="grid grid-cols-3 gap-4">
        <div className="flex flex-col gap-2">
          <Label>{t("form.type")}</Label>
          <Select
            value={form.type}
            onValueChange={(v) => onChange({ ...form, type: v as RuleFormData["type"] })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="keyword">{t("type.keyword")}</SelectItem>
              <SelectItem value="regex">{t("type.regex")}</SelectItem>
              <SelectItem value="length">{t("type.length")}</SelectItem>
              <SelectItem value="pii">{t("type.pii")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-2">
          <Label>{t("form.target")}</Label>
          <Select
            value={form.target}
            onValueChange={(v) => onChange({ ...form, target: v as RuleFormData["target"] })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="input">{t("target.input")}</SelectItem>
              <SelectItem value="output">{t("target.output")}</SelectItem>
              <SelectItem value="both">{t("target.both")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
        <div className="flex flex-col gap-2">
          <Label>{t("form.action")}</Label>
          <Select
            value={form.action}
            onValueChange={(v) => onChange({ ...form, action: v as RuleFormData["action"] })}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="block">{t("action.block")}</SelectItem>
              <SelectItem value="warn">{t("action.warn")}</SelectItem>
              <SelectItem value="redact">{t("action.redact")}</SelectItem>
            </SelectContent>
          </Select>
        </div>
      </div>
      {form.type === "length" ? (
        <div className="flex flex-col gap-2">
          <Label>{t("form.maxLength")}</Label>
          <Input
            type="number"
            min="1"
            value={form.maxLength}
            onChange={(e) => onChange({ ...form, maxLength: e.target.value })}
          />
          <p className="text-muted-foreground text-xs">{t("form.maxLengthHint")}</p>
        </div>
      ) : form.type !== "pii" ? (
        <div className="flex flex-col gap-2">
          <Label>{t("form.pattern")}</Label>
          <Textarea
            value={form.pattern}
            onChange={(e) => onChange({ ...form, pattern: e.target.value })}
            placeholder={t("form.patternPlaceholder")}
            rows={3}
          />
          <p className="text-muted-foreground text-xs">{t("form.patternHint")}</p>
        </div>
      ) : null}
      <div className="flex items-center gap-2">
        <Switch
          id="rule-enabled"
          checked={form.enabled}
          onCheckedChange={(checked) => onChange({ ...form, enabled: checked })}
        />
        <Label htmlFor="rule-enabled">{t("form.enabled")}</Label>
      </div>
      <Button type="submit">{t("actions.save", { ns: "common" })}</Button>
    </form>
  )
}

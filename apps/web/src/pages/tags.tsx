import { Pencil, Plus, Tags, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
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
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"

// Local state management (no backend API yet — UI-ready scaffold)

interface Tag {
  id: string
  name: string
  color: string
  description: string
  channelCount: number
  keyCount: number
  createdAt: string
}

interface TagFormData {
  name: string
  color: string
  description: string
}

const EMPTY_FORM: TagFormData = {
  name: "",
  color: "blue",
  description: "",
}

const TAG_COLORS = ["gray", "red", "orange", "yellow", "green", "blue", "purple", "pink"] as const

const colorClasses: Record<string, string> = {
  gray: "bg-gray-100 text-gray-800 dark:bg-gray-800 dark:text-gray-200",
  red: "bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200",
  orange: "bg-orange-100 text-orange-800 dark:bg-orange-900 dark:text-orange-200",
  yellow: "bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200",
  green: "bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200",
  blue: "bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200",
  purple: "bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200",
  pink: "bg-pink-100 text-pink-800 dark:bg-pink-900 dark:text-pink-200",
}

export default function TagsPage() {
  const { t } = useTranslation("tags")
  const [tags, setTags] = useState<Tag[]>([])
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<TagFormData>(EMPTY_FORM)
  const [editingTag, setEditingTag] = useState<Tag | null>(null)
  const [editForm, setEditForm] = useState<TagFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<Tag | null>(null)

  function handleCreate(form: TagFormData) {
    const tag: Tag = {
      id: crypto.randomUUID(),
      name: form.name,
      color: form.color,
      description: form.description,
      channelCount: 0,
      keyCount: 0,
      createdAt: new Date().toISOString(),
    }
    setTags((prev) => [...prev, tag])
    setCreateForm(EMPTY_FORM)
    setShowCreate(false)
    toast.success(t("tagSaved"))
  }

  function handleUpdate(id: string, form: TagFormData) {
    setTags((prev) =>
      prev.map((tag) =>
        tag.id === id
          ? { ...tag, name: form.name, color: form.color, description: form.description }
          : tag,
      ),
    )
    setEditingTag(null)
    toast.success(t("tagSaved"))
  }

  function handleDelete(id: string) {
    setTags((prev) => prev.filter((tag) => tag.id !== id))
    setDeleteConfirm(null)
    toast.success(t("tagDeleted"))
  }

  function openEdit(tag: Tag) {
    setEditForm({ name: tag.name, color: tag.color, description: tag.description })
    setEditingTag(tag)
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
                <Plus className="mr-2 h-4 w-4" /> {t("addTag")}
              </Button>
            </DialogTrigger>
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("addTag")}</DialogTitle>
              </DialogHeader>
              <TagForm
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
            open={!!editingTag}
            onOpenChange={(open) => {
              if (!open) setEditingTag(null)
            }}
          >
            <DialogContent>
              <DialogHeader>
                <DialogTitle>{t("editTag")}</DialogTitle>
              </DialogHeader>
              <TagForm
                form={editForm}
                onChange={setEditForm}
                onSubmit={() => editingTag && handleUpdate(editingTag.id, editForm)}
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

          {tags.length === 0 ? (
            <div className="flex flex-col items-center justify-center gap-2 py-16">
              <Tags className="text-muted-foreground h-10 w-10" />
              <p className="text-muted-foreground font-medium">{t("noTags")}</p>
              <p className="text-muted-foreground text-sm">{t("noTagsHint")}</p>
            </div>
          ) : (
            <div className="overflow-x-auto">
              <Table>
                <TableHeader>
                  <TableRow>
                    <TableHead>{t("table.name")}</TableHead>
                    <TableHead>{t("table.color")}</TableHead>
                    <TableHead className="text-right">{t("table.channels")}</TableHead>
                    <TableHead className="text-right">{t("table.keys")}</TableHead>
                    <TableHead className="w-24" />
                  </TableRow>
                </TableHeader>
                <TableBody>
                  {tags.map((tag) => (
                    <TableRow key={tag.id}>
                      <TableCell>
                        <div className="flex flex-col">
                          <span className="font-medium">{tag.name}</span>
                          {tag.description && (
                            <span className="text-muted-foreground text-xs">{tag.description}</span>
                          )}
                        </div>
                      </TableCell>
                      <TableCell>
                        <span
                          className={`inline-flex rounded-full px-2 py-0.5 text-xs font-medium ${colorClasses[tag.color] || colorClasses.blue}`}
                        >
                          {t(`colors.${tag.color}`)}
                        </span>
                      </TableCell>
                      <TableCell className="text-right font-mono">{tag.channelCount}</TableCell>
                      <TableCell className="text-right font-mono">{tag.keyCount}</TableCell>
                      <TableCell>
                        <div className="flex gap-1">
                          <Button variant="ghost" size="icon" onClick={() => openEdit(tag)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => setDeleteConfirm(tag)}>
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

function TagForm({
  form,
  onChange,
  onSubmit,
}: {
  form: TagFormData
  onChange: (f: TagFormData) => void
  onSubmit: () => void
}) {
  const { t } = useTranslation("tags")
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
      <div className="flex flex-col gap-2">
        <Label>{t("form.color")}</Label>
        <Select value={form.color} onValueChange={(v) => onChange({ ...form, color: v })}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            {TAG_COLORS.map((color) => (
              <SelectItem key={color} value={color}>
                <span className="flex items-center gap-2">
                  <span
                    className={`inline-block h-3 w-3 rounded-full ${colorClasses[color]?.split(" ")[0] || "bg-blue-100"}`}
                  />
                  {t(`colors.${color}`)}
                </span>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("form.description")}</Label>
        <Textarea
          value={form.description}
          onChange={(e) => onChange({ ...form, description: e.target.value })}
          placeholder={t("form.descriptionPlaceholder")}
          rows={2}
        />
      </div>
      <Button type="submit">{t("actions.save", { ns: "common" })}</Button>
    </form>
  )
}

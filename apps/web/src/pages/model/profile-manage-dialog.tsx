import type { ModelProfile, ProfilePreviewGroup } from "@/lib/api-client"
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { ArrowLeft, Loader2, Pencil, Plus, Shield, Trash2 } from "lucide-react"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import { Dialog, DialogContent, DialogHeader, DialogTitle } from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  useCreateProfile,
  useDeleteProfile,
  useProfilesQuery,
  useUpdateProfile,
} from "@/hooks/use-profiles"
import { deleteGroup, listProfileGroupsPreview, materializeProfileGroups } from "@/lib/api-client"

// ─── Profile Form ────────────────────────────

interface ProfileFormProps {
  name: string
  onChangeName: (name: string) => void
  onSubmit: () => void
  onCancel: () => void
  isPending: boolean
  submitLabel: string
}

function ProfileForm({
  name,
  onChangeName,
  onSubmit,
  onCancel,
  isPending,
  submitLabel,
}: ProfileFormProps) {
  const { t } = useTranslation("model")

  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-3 rounded-md border p-3"
    >
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("profile.form.name")}</Label>
        <Input
          value={name}
          onChange={(e) => onChangeName(e.target.value)}
          placeholder={t("profile.form.namePlaceholder")}
          required
        />
      </div>

      <div className="flex items-center gap-2">
        <Button type="submit" size="sm" disabled={isPending}>
          {isPending && <Loader2 className="mr-1 h-3 w-3 animate-spin" />}
          {submitLabel}
        </Button>
        <Button type="button" variant="ghost" size="sm" onClick={onCancel}>
          {t("actions.cancel", { ns: "common" })}
        </Button>
      </div>
    </form>
  )
}

// ─── Profile Card ────────────────────────────

function ProfileCard({
  profile,
  onEdit,
  onDelete,
  onMaterialize,
}: {
  profile: ModelProfile
  onEdit: () => void
  onDelete: () => void
  onMaterialize?: () => void
}) {
  const { t } = useTranslation("model")

  return (
    <div
      className={`flex items-center justify-between rounded-md border p-3 ${profile.isBuiltin ? "hover:bg-accent/50 cursor-pointer transition-colors" : ""}`}
      onClick={profile.isBuiltin ? onMaterialize : undefined}
    >
      <div className="flex items-center gap-2 overflow-hidden">
        <div className="min-w-0">
          <span className="truncate text-sm font-medium">{profile.name}</span>
          <p className="text-muted-foreground text-[11px]">
            {t("modelCount", { count: profile.groupCount ?? 0 })}
          </p>
        </div>
        {profile.isBuiltin && (
          <Badge variant="secondary" className="shrink-0 text-[10px]">
            <Shield className="mr-0.5 h-2.5 w-2.5" />
            {t("profile.builtin")}
          </Badge>
        )}
      </div>
      <div className="flex shrink-0 items-center gap-0.5">
        {!profile.isBuiltin && (
          <>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={onEdit}
              title={t("actions.edit", { ns: "common" })}
            >
              <Pencil className="h-3 w-3" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={onDelete}
              title={t("actions.delete", { ns: "common" })}
            >
              <Trash2 className="text-destructive h-3 w-3" />
            </Button>
          </>
        )}
      </div>
    </div>
  )
}

// ─── Materialize View ────────────────────────

function MaterializeView({ profile, onBack }: { profile: ModelProfile; onBack: () => void }) {
  const { t } = useTranslation("model")
  const queryClient = useQueryClient()
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: previewData, isLoading: previewLoading } = useQuery({
    queryKey: ["profile-group-preview", profile.id],
    queryFn: () => listProfileGroupsPreview(profile.id),
  })

  const previewGroups = useMemo(
    () => (previewData?.data?.groups ?? []) as ProfilePreviewGroup[],
    [previewData],
  )
  useEffect(() => {
    // 默认选中所有未落地项
    const defaults = previewGroups.filter((g) => !g.materialized).map((g) => g.model)
    setSelected(new Set(defaults))
  }, [previewGroups])

  function invalidate() {
    queryClient.invalidateQueries({
      predicate: (q) => {
        const key = q.queryKey[0]
        return key === "groups" || key === "profile-group-preview" || key === "profiles"
      },
    })
  }

  const materializeMut = useMutation({
    mutationFn: (models?: string[]) =>
      materializeProfileGroups(profile.id, models ? { models } : {}),
    onSuccess: () => {
      invalidate()
      toast.success(t("profile.toast.materialized"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const unmaterializeAllMut = useMutation({
    mutationFn: async (groupIds: number[]) => {
      await Promise.all(groupIds.map((id) => deleteGroup(id)))
    },
    onSuccess: () => {
      invalidate()
      toast.success(t("profile.toast.unmaterializedAll"))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  function toggleSelection(model: string, checked: boolean) {
    setSelected((prev) => {
      const next = new Set(prev)
      if (checked) next.add(model)
      else next.delete(model)
      return next
    })
  }

  // 全选 checkbox 状态：基于所有项（不区分落地状态）
  const selectAllState: boolean | "indeterminate" = useMemo(() => {
    if (previewGroups.length === 0) return false
    const selectedCount = previewGroups.filter((g) => selected.has(g.model)).length
    if (selectedCount === 0) return false
    if (selectedCount === previewGroups.length) return true
    return "indeterminate"
  }, [previewGroups, selected])

  // 选中项中：未落地 / 已落地的数量，用于按钮启用状态
  const selectedUnmaterialized = useMemo(
    () => previewGroups.filter((g) => selected.has(g.model) && !g.materialized),
    [previewGroups, selected],
  )
  const selectedMaterialized = useMemo(
    () => previewGroups.filter((g) => selected.has(g.model) && g.materialized && g.groupId),
    [previewGroups, selected],
  )

  function toggleSelectAll() {
    if (selectAllState === true) {
      setSelected(new Set())
    } else {
      setSelected(new Set(previewGroups.map((g) => g.model)))
    }
  }

  function materializeSelected() {
    const models = selectedUnmaterialized.map((g) => g.model)
    if (models.length > 0) materializeMut.mutate(models)
  }

  function unmaterializeSelected() {
    const groupIds = selectedMaterialized.map((g) => g.groupId!)
    if (groupIds.length > 0) unmaterializeAllMut.mutate(groupIds)
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <Button variant="ghost" size="icon" className="h-7 w-7" onClick={onBack}>
          <ArrowLeft className="h-4 w-4" />
        </Button>
        <span className="text-sm font-medium">{profile.name}</span>
        <Badge variant="secondary" className="text-[10px]">
          <Shield className="mr-0.5 h-2.5 w-2.5" />
          {t("profile.builtin")}
        </Badge>
      </div>

      <div className="flex items-center gap-2">
        <label className="flex cursor-pointer items-center gap-1.5 text-sm">
          <Checkbox
            checked={selectAllState}
            onCheckedChange={() => toggleSelectAll()}
            disabled={previewGroups.length === 0}
          />
          <span className="text-muted-foreground text-xs select-none">
            {selectAllState === true ? t("profile.form.deselectAll") : t("profile.form.selectAll")}
          </span>
        </label>
        <div className="ml-auto flex items-center gap-1.5">
          <Button
            size="sm"
            onClick={materializeSelected}
            disabled={materializeMut.isPending || selectedUnmaterialized.length === 0}
          >
            {t("profile.materialize")}
          </Button>
          <Button
            size="sm"
            variant="outline"
            onClick={unmaterializeSelected}
            disabled={unmaterializeAllMut.isPending || selectedMaterialized.length === 0}
          >
            {t("profile.unmaterialize")}
          </Button>
        </div>
      </div>

      <ScrollArea className="max-h-[50vh]">
        {previewLoading ? (
          <div className="flex justify-center py-8">
            <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
          </div>
        ) : previewGroups.length === 0 ? (
          <p className="text-muted-foreground py-8 text-center text-sm">
            {t("profile.previewEmpty")}
          </p>
        ) : (
          <div className="flex flex-col gap-2">
            {previewGroups.map((g) => (
              <div key={g.key} className="flex items-center gap-2 rounded-md border p-2">
                <Checkbox
                  checked={selected.has(g.model)}
                  onCheckedChange={(checked) => toggleSelection(g.model, checked === true)}
                />
                <div className="min-w-0 flex-1">
                  <p className="truncate text-sm font-medium">{g.name}</p>
                  <div className="text-muted-foreground flex items-center gap-1 text-xs">
                    <span>{g.model}</span>
                    {g.materialized ? (
                      <Badge variant="secondary" className="text-[10px]">
                        {t("profile.materialized")}
                      </Badge>
                    ) : (
                      <Badge variant="outline" className="text-[10px]">
                        {t("profile.virtual")}
                      </Badge>
                    )}
                  </div>
                </div>
                {!g.materialized ? (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => materializeMut.mutate([g.model])}
                    disabled={materializeMut.isPending}
                  >
                    {t("profile.materialize")}
                  </Button>
                ) : (
                  <Button
                    size="sm"
                    variant="ghost"
                    onClick={() => {
                      if (!g.groupId) return
                      unmaterializeAllMut.mutate([g.groupId])
                    }}
                  >
                    {t("profile.unmaterialize")}
                  </Button>
                )}
              </div>
            ))}
          </div>
        )}
      </ScrollArea>
    </div>
  )
}

// ─── Main Dialog ─────────────────────────────

interface ProfileManageDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

export default function ProfileManageDialog({ open, onOpenChange }: ProfileManageDialogProps) {
  const { t } = useTranslation("model")
  const { data, isLoading } = useProfilesQuery()
  const createMut = useCreateProfile()
  const updateMut = useUpdateProfile()
  const deleteMut = useDeleteProfile()

  const [formMode, setFormMode] = useState<"hidden" | "create" | "edit">("hidden")
  const [editingId, setEditingId] = useState<number | null>(null)
  const [name, setName] = useState("")
  const [materializeProfile, setMaterializeProfile] = useState<ModelProfile | null>(null)

  const profiles = useMemo(() => (data?.data?.profiles ?? []) as ModelProfile[], [data])
  const builtinProfiles = useMemo(() => profiles.filter((p) => p.isBuiltin), [profiles])
  const customProfiles = useMemo(() => profiles.filter((p) => !p.isBuiltin), [profiles])

  function openCreate() {
    setName("")
    setEditingId(null)
    setFormMode("create")
  }

  function openEdit(p: ModelProfile) {
    setName(p.name)
    setEditingId(p.id)
    setFormMode("edit")
  }

  function handleSubmit() {
    if (formMode === "create") {
      createMut.mutate(
        { name },
        {
          onSuccess: () => {
            setFormMode("hidden")
            toast.success(t("profile.toast.created"))
          },
          onError: (err) => toast.error(err.message),
        },
      )
    } else if (formMode === "edit" && editingId) {
      updateMut.mutate(
        { id: editingId, name },
        {
          onSuccess: () => {
            setFormMode("hidden")
            toast.success(t("profile.toast.updated"))
          },
          onError: (err) => toast.error(err.message),
        },
      )
    }
  }

  function handleDelete(p: ModelProfile) {
    deleteMut.mutate(p.id, {
      onSuccess: () => toast.success(t("profile.toast.deleted")),
      onError: (err) => toast.error(err.message),
    })
  }

  return (
    <Dialog
      open={open}
      onOpenChange={(v) => {
        if (!v) setMaterializeProfile(null)
        onOpenChange(v)
      }}
    >
      <DialogContent className="max-h-[85vh] w-full max-w-lg overflow-hidden">
        {materializeProfile ? (
          <MaterializeView
            profile={materializeProfile}
            onBack={() => setMaterializeProfile(null)}
          />
        ) : (
          <>
            <DialogHeader>
              <DialogTitle>{t("profile.title")}</DialogTitle>
            </DialogHeader>

            <div className="flex flex-col gap-3">
              {formMode === "hidden" && (
                <Button variant="outline" size="sm" onClick={openCreate}>
                  <Plus className="mr-1 h-3 w-3" />
                  {t("profile.createProfile")}
                </Button>
              )}

              {formMode !== "hidden" && (
                <ProfileForm
                  name={name}
                  onChangeName={setName}
                  onSubmit={handleSubmit}
                  onCancel={() => setFormMode("hidden")}
                  isPending={createMut.isPending || updateMut.isPending}
                  submitLabel={
                    formMode === "edit"
                      ? t("actions.save", { ns: "common" })
                      : t("actions.create", { ns: "common" })
                  }
                />
              )}

              <ScrollArea className="max-h-[50vh]">
                {isLoading ? (
                  <div className="flex justify-center py-8">
                    <Loader2 className="text-muted-foreground h-5 w-5 animate-spin" />
                  </div>
                ) : profiles.length === 0 ? (
                  <p className="text-muted-foreground py-8 text-center text-sm">
                    {t("profile.empty")}
                  </p>
                ) : (
                  <div className="flex flex-col gap-2">
                    {builtinProfiles.length > 0 && (
                      <div className="flex flex-col gap-1.5">
                        <span className="text-muted-foreground text-xs font-medium">
                          {t("profile.builtinSection")}
                        </span>
                        {builtinProfiles.map((p) => (
                          <ProfileCard
                            key={p.id}
                            profile={p}
                            onEdit={() => {}}
                            onDelete={() => {}}
                            onMaterialize={() => setMaterializeProfile(p)}
                          />
                        ))}
                      </div>
                    )}
                    {customProfiles.length > 0 && (
                      <div className="flex flex-col gap-1.5">
                        <span className="text-muted-foreground text-xs font-medium">
                          {t("profile.customSection")}
                        </span>
                        {customProfiles.map((p) => (
                          <ProfileCard
                            key={p.id}
                            profile={p}
                            onEdit={() => openEdit(p)}
                            onDelete={() => handleDelete(p)}
                          />
                        ))}
                      </div>
                    )}
                  </div>
                )}
              </ScrollArea>
            </div>
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

import type { Team, TeamBudgetSummary } from "@/lib/api"
import { DollarSign, Pencil, Plus, Trash2, Users } from "lucide-react"
import { useCallback, useEffect, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { ConfirmDeleteDialog } from "@/components/confirm-delete-dialog"
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
import { Progress } from "@/components/ui/progress"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Textarea } from "@/components/ui/textarea"
import {
  createTeam,
  deleteTeam,
  getTeamBudgets,
  listTeams,
  updateTeam,
} from "@/lib/api"

interface TeamFormData {
  name: string
  description: string
  maxBudget: string
}

const EMPTY_FORM: TeamFormData = {
  name: "",
  description: "",
  maxBudget: "",
}

export default function TeamsPage() {
  const { t } = useTranslation("teams")
  const [teams, setTeams] = useState<Team[]>([])
  const [budgets, setBudgets] = useState<TeamBudgetSummary[]>([])
  const [loading, setLoading] = useState(true)
  const [showCreate, setShowCreate] = useState(false)
  const [createForm, setCreateForm] = useState<TeamFormData>(EMPTY_FORM)
  const [editingTeam, setEditingTeam] = useState<Team | null>(null)
  const [editForm, setEditForm] = useState<TeamFormData>(EMPTY_FORM)
  const [deleteConfirm, setDeleteConfirm] = useState<Team | null>(null)

  const fetchData = useCallback(async () => {
    try {
      const [teamsRes, budgetsRes] = await Promise.all([listTeams(), getTeamBudgets()])
      setTeams(teamsRes.data.teams)
      setBudgets(budgetsRes.data.budgets)
    } catch {
      toast.error(t("fetchError", { ns: "common", defaultValue: "Failed to fetch data" }))
    } finally {
      setLoading(false)
    }
  }, [t])

  useEffect(() => {
    fetchData()
  }, [fetchData])

  const handleCreate = async () => {
    try {
      await createTeam({
        name: createForm.name,
        description: createForm.description,
        maxBudget: createForm.maxBudget ? Number(createForm.maxBudget) : 0,
      })
      toast.success(t("createSuccess"))
      setShowCreate(false)
      setCreateForm(EMPTY_FORM)
      fetchData()
    } catch {
      toast.error(t("createError"))
    }
  }

  const handleUpdate = async () => {
    if (!editingTeam) return
    try {
      await updateTeam({
        id: editingTeam.id,
        name: editForm.name,
        description: editForm.description,
        maxBudget: editForm.maxBudget ? Number(editForm.maxBudget) : 0,
      })
      toast.success(t("updateSuccess"))
      setEditingTeam(null)
      fetchData()
    } catch {
      toast.error(t("updateError"))
    }
  }

  const handleDelete = async () => {
    if (!deleteConfirm) return
    try {
      await deleteTeam(deleteConfirm.id)
      toast.success(t("deleteSuccess"))
      setDeleteConfirm(null)
      fetchData()
    } catch {
      toast.error(t("deleteError"))
    }
  }

  const getBudget = (teamId: number) => budgets.find((b) => b.teamId === teamId)

  if (loading) {
    return (
      <div className="flex min-h-0 flex-1 items-center justify-center">
        <div className="text-muted-foreground text-sm">{t("loading", { ns: "common", defaultValue: "Loading..." })}</div>
      </div>
    )
  }

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="flex shrink-0 items-center justify-between pb-4">
        <div>
          <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
          <p className="text-muted-foreground text-sm">{t("description")}</p>
        </div>
        <Dialog open={showCreate} onOpenChange={setShowCreate}>
          <DialogTrigger asChild>
            <Button>
              <Plus className="mr-1 h-4 w-4" /> {t("addTeam")}
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>{t("addTeam")}</DialogTitle>
            </DialogHeader>
            <TeamForm form={createForm} setForm={setCreateForm} onSave={handleCreate} t={t} />
          </DialogContent>
        </Dialog>
      </div>

      {/* Budget overview cards */}
      {budgets.length > 0 && (
        <div className="mb-4 grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3">
          {budgets.map((b) => (
            <Card key={b.teamId}>
              <CardContent className="flex flex-col gap-2 p-4">
                <div className="flex items-center justify-between">
                  <span className="font-medium">{b.teamName}</span>
                  <Badge variant="secondary">
                    <Users className="mr-1 h-3 w-3" />
                    {b.virtualKeys} {t("virtualKeys")}
                  </Badge>
                </div>
                <div className="flex items-center gap-2 text-sm">
                  <DollarSign className="text-muted-foreground h-3.5 w-3.5" />
                  <span>
                    ${b.totalSpend.toFixed(2)} / ${b.maxBudget > 0 ? b.maxBudget.toFixed(2) : "∞"}
                  </span>
                </div>
                {b.maxBudget > 0 && (
                  <Progress value={Math.min(b.budgetUsedPercent, 100)} className="h-2" />
                )}
              </CardContent>
            </Card>
          ))}
        </div>
      )}

      {/* Teams table */}
      <div className="overflow-auto rounded-md border">
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>ID</TableHead>
              <TableHead>{t("name")}</TableHead>
              <TableHead>{t("descriptionLabel")}</TableHead>
              <TableHead>{t("maxBudget")}</TableHead>
              <TableHead>{t("spend")}</TableHead>
              <TableHead className="w-24" />
            </TableRow>
          </TableHeader>
          <TableBody>
            {teams.length === 0 ? (
              <TableRow>
                <TableCell colSpan={6} className="text-muted-foreground text-center">
                  {t("empty")}
                </TableCell>
              </TableRow>
            ) : (
              teams.map((team) => {
                const budget = getBudget(team.id)
                return (
                  <TableRow key={team.id}>
                    <TableCell className="font-mono text-xs">{team.id}</TableCell>
                    <TableCell className="font-medium">{team.name}</TableCell>
                    <TableCell className="text-muted-foreground max-w-[200px] truncate text-sm">
                      {team.description || "—"}
                    </TableCell>
                    <TableCell>
                      {team.maxBudget > 0 ? `$${team.maxBudget.toFixed(2)}` : "—"}
                    </TableCell>
                    <TableCell>
                      {budget ? `$${budget.totalSpend.toFixed(2)}` : "—"}
                    </TableCell>
                    <TableCell>
                      <div className="flex gap-1">
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => {
                            setEditingTeam(team)
                            setEditForm({
                              name: team.name,
                              description: team.description,
                              maxBudget: team.maxBudget > 0 ? String(team.maxBudget) : "",
                            })
                          }}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        <Button
                          variant="ghost"
                          size="icon"
                          onClick={() => setDeleteConfirm(team)}
                        >
                          <Trash2 className="h-4 w-4" />
                        </Button>
                      </div>
                    </TableCell>
                  </TableRow>
                )
              })
            )}
          </TableBody>
        </Table>
      </div>

      {/* Edit dialog */}
      <Dialog open={!!editingTeam} onOpenChange={(open) => !open && setEditingTeam(null)}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>{t("editTeam")}</DialogTitle>
          </DialogHeader>
          <TeamForm form={editForm} setForm={setEditForm} onSave={handleUpdate} t={t} />
        </DialogContent>
      </Dialog>

      {/* Delete confirm */}
      <ConfirmDeleteDialog
        open={!!deleteConfirm}
        onOpenChange={(open) => !open && setDeleteConfirm(null)}
        onConfirm={handleDelete}
        title={t("deleteConfirmTitle")}
        description={t("deleteConfirmDesc", { name: deleteConfirm?.name })}
      />
    </div>
  )
}

function TeamForm({
  form,
  setForm,
  onSave,
  t,
}: {
  form: TeamFormData
  setForm: (f: TeamFormData) => void
  onSave: () => void
  t: (key: string, opts?: Record<string, unknown>) => string
}) {
  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col gap-2">
        <Label>{t("name")}</Label>
        <Input
          value={form.name}
          onChange={(e) => setForm({ ...form, name: e.target.value })}
          placeholder={t("namePlaceholder")}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("descriptionLabel")}</Label>
        <Textarea
          value={form.description}
          onChange={(e) => setForm({ ...form, description: e.target.value })}
          placeholder={t("descriptionPlaceholder")}
          rows={3}
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>{t("maxBudget")}</Label>
        <Input
          type="number"
          value={form.maxBudget}
          onChange={(e) => setForm({ ...form, maxBudget: e.target.value })}
          placeholder={t("maxBudgetPlaceholder")}
        />
      </div>
      <Button onClick={onSave} disabled={!form.name.trim()}>
        {t("save", { ns: "common", defaultValue: "Save" })}
      </Button>
    </div>
  )
}

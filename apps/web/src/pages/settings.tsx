import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Pencil, Plus, Trash2 } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { lazy, Suspense, useState } from "react"
import { toast } from "sonner"
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
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  changePassword,
  changeUsername,
  createApiKey,
  deleteApiKey,
  listApiKeys,
  updateApiKey,
} from "@/lib/api"

// ───────────── Lazy-loaded sections ─────────────

const SystemConfigSection = lazy(() => import("./settings/system-config-section"))

const BackupSection = lazy(() => import("./settings/backup-section"))

// ───────────── API Keys types ─────────────

interface ApiKeyRecord {
  id: number
  name: string
  apiKey: string
  enabled: boolean
  expireAt: number
  maxCost: number
  totalCost: number
  supportedModels: string
}

interface ApiKeyFormData {
  name: string
  expireAt: string
  maxCost: string
  supportedModels: string
}

const EMPTY_FORM: ApiKeyFormData = {
  name: "",
  expireAt: "",
  maxCost: "",
  supportedModels: "",
}

// ───────────── Account Section ─────────────

function AccountSection() {
  const [newUsername, setNewUsername] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")

  const usernameMutation = useMutation({
    mutationFn: () => changeUsername(newUsername),
    onSuccess: () => {
      toast.success("Username updated")
      setNewUsername("")
    },
    onError: () => toast.error("Failed to update username"),
  })

  const passwordMutation = useMutation({
    mutationFn: () => changePassword(newPassword),
    onSuccess: () => {
      toast.success("Password updated")
      setNewPassword("")
      setConfirmPassword("")
    },
    onError: () => toast.error("Failed to update password"),
  })

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>Change Username</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!newUsername.trim()) return
              usernameMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>New Username</Label>
              <Input
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder="Enter new username"
                required
              />
            </div>
            <Button type="submit" disabled={usernameMutation.isPending}>
              Update Username
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Change Password</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (newPassword.length < 8) {
                toast.error("Password must be at least 8 characters")
                return
              }
              if (newPassword !== confirmPassword) {
                toast.error("Passwords do not match")
                return
              }
              if (!newPassword) return
              passwordMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>New Password</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Enter new password"
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>Confirm Password</Label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Confirm new password"
                required
              />
            </div>
            <Button type="submit" disabled={passwordMutation.isPending}>
              Update Password
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

// ───────────── API Keys Section ─────────────

function ApiKeysSection() {
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
      }),
    onSuccess: (res) => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      const key = (res.data as { apiKey?: string })?.apiKey
      if (key) setCreatedKey(key)
      setCreateForm(EMPTY_FORM)
    },
  })

  const updateMutation = useMutation({
    mutationFn: ({ id, form }: { id: number; form: ApiKeyFormData }) =>
      updateApiKey({
        id,
        name: form.name,
        expireAt: form.expireAt ? Math.floor(new Date(form.expireAt).getTime() / 1000) : 0,
        maxCost: form.maxCost ? Number.parseFloat(form.maxCost) : 0,
        supportedModels: form.supportedModels,
      }),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      setEditingKey(null)
      toast.success("API Key updated")
    },
  })

  const deleteMutation = useMutation({
    mutationFn: deleteApiKey,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["apikeys"] })
      toast.success("API Key deleted")
    },
  })

  const apiKeys = (data?.data?.apiKeys ?? []) as ApiKeyRecord[]

  function maskKey(key: string) {
    if (key.length <= 16) return key
    return `${key.slice(0, 16)}...${key.slice(-4)}`
  }

  function formatExpiry(timestamp: number) {
    if (!timestamp) return "Never"
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
    })
    setEditingKey(k)
  }

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>API Keys</CardTitle>
        <Dialog
          open={showCreate}
          onOpenChange={(open) => {
            if (!open && createdKey && !keyCopied) {
              toast.error("Please copy the key before closing")
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
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="mr-2 h-4 w-4" /> Create Key
            </Button>
          </DialogTrigger>
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Create API Key</DialogTitle>
            </DialogHeader>
            {createdKey ? (
              <div className="flex flex-col gap-3">
                <p className="text-muted-foreground text-sm">
                  Save this key — it won&apos;t be shown again.
                </p>
                <div className="bg-muted flex items-center gap-2 rounded-md p-3">
                  <code className="flex-1 text-sm break-all">{createdKey}</code>
                  <Button
                    variant="ghost"
                    size="icon"
                    onClick={() => {
                      navigator.clipboard.writeText(createdKey)
                      setKeyCopied(true)
                      toast.success("Copied!")
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
                  Done
                </Button>
              </div>
            ) : (
              <ApiKeyForm
                form={createForm}
                onChange={setCreateForm}
                onSubmit={() => createMutation.mutate(createForm)}
                isPending={createMutation.isPending}
                submitLabel="Create"
              />
            )}
          </DialogContent>
        </Dialog>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {/* Edit Dialog */}
        <Dialog
          open={!!editingKey}
          onOpenChange={(open) => {
            if (!open) setEditingKey(null)
          }}
        >
          <DialogContent>
            <DialogHeader>
              <DialogTitle>Edit API Key</DialogTitle>
            </DialogHeader>
            <ApiKeyForm
              form={editForm}
              onChange={setEditForm}
              onSubmit={() => {
                if (editingKey) {
                  updateMutation.mutate({ id: editingKey.id, form: editForm })
                }
              }}
              isPending={updateMutation.isPending}
              submitLabel="Save"
            />
          </DialogContent>
        </Dialog>

        {/* Delete Confirmation */}
        <AlertDialog
          open={!!deleteConfirm}
          onOpenChange={(open) => !open && setDeleteConfirm(null)}
        >
          <AlertDialogContent>
            <AlertDialogHeader>
              <AlertDialogTitle>
                Delete API Key &ldquo;{deleteConfirm?.name}&rdquo;?
              </AlertDialogTitle>
              <AlertDialogDescription>
                This action cannot be undone. Any applications using this key will lose access
                immediately.
              </AlertDialogDescription>
            </AlertDialogHeader>
            <AlertDialogFooter>
              <AlertDialogCancel>Cancel</AlertDialogCancel>
              <AlertDialogAction
                variant="destructive"
                onClick={() => {
                  if (deleteConfirm) deleteMutation.mutate(deleteConfirm.id)
                  setDeleteConfirm(null)
                }}
              >
                Delete
              </AlertDialogAction>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialog>

        {isLoading ? (
          <p className="text-muted-foreground">Loading...</p>
        ) : (
          <div className="overflow-x-auto">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Name</TableHead>
                  <TableHead>Key</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Expires</TableHead>
                  <TableHead>Cost / Limit</TableHead>
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
                              toast.success("Copied!")
                            }}
                          >
                            <Copy className="h-3 w-3" />
                          </Button>
                        </div>
                      </TableCell>
                      <TableCell>
                        <Badge variant={k.enabled ? "default" : "secondary"}>
                          {k.enabled ? "Active" : "Disabled"}
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
                        <div className="flex gap-1">
                          <Button variant="ghost" size="icon" onClick={() => openEdit(k)}>
                            <Pencil className="h-4 w-4" />
                          </Button>
                          <Button variant="ghost" size="icon" onClick={() => setDeleteConfirm(k)}>
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
      </CardContent>
    </Card>
  )
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
  return (
    <form
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
      className="flex flex-col gap-4"
    >
      <div className="flex flex-col gap-2">
        <Label>Name</Label>
        <Input
          value={form.name}
          onChange={(e) => onChange({ ...form, name: e.target.value })}
          placeholder="My API Key"
          required
        />
      </div>
      <div className="flex flex-col gap-2">
        <Label>Expire Date</Label>
        <Input
          type="date"
          value={form.expireAt}
          onChange={(e) => onChange({ ...form, expireAt: e.target.value })}
        />
        <p className="text-muted-foreground text-xs">Leave empty for no expiry</p>
      </div>
      <div className="flex flex-col gap-2">
        <Label>Cost Limit ($)</Label>
        <Input
          type="number"
          step="0.01"
          min="0"
          value={form.maxCost}
          onChange={(e) => onChange({ ...form, maxCost: e.target.value })}
          placeholder="0 = unlimited"
        />
        <p className="text-muted-foreground text-xs">0 or empty = unlimited</p>
      </div>
      <div className="flex flex-col gap-2">
        <Label>Model Whitelist</Label>
        <Input
          value={form.supportedModels}
          onChange={(e) => onChange({ ...form, supportedModels: e.target.value })}
          placeholder="gpt-4o, claude-3.5-sonnet (comma-separated)"
        />
        <p className="text-muted-foreground text-xs">
          Comma-separated model names. Empty = all models allowed.
        </p>
      </div>
      <Button type="submit" disabled={isPending}>
        {submitLabel}
      </Button>
    </form>
  )
}

// ───────────── Settings Page ─────────────

export default function SettingsPage() {
  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-2xl font-bold tracking-tight">Settings</h2>
      <ApiKeysSection />
      <AccountSection />
      <Suspense fallback={<p className="text-muted-foreground">Loading...</p>}>
        <SystemConfigSection />
      </Suspense>
      <Suspense fallback={<p className="text-muted-foreground">Loading...</p>}>
        <BackupSection />
      </Suspense>
    </div>
  )
}

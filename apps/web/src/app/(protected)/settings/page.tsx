"use client"

import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { Copy, Download, FileUp, Pencil, Plus, Trash2, Upload } from "lucide-react"
import { AnimatePresence, motion } from "motion/react"
import { useCallback, useRef, useState } from "react"
import { toast } from "sonner"
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
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import {
  changePassword,
  changeUsername,
  createApiKey,
  deleteApiKey,
  exportData,
  getSettings,
  importData,
  listApiKeys,
  updateApiKey,
  updateSettings,
} from "@/lib/api"

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

// ───────────── System Config Section ─────────────

function SystemConfigSection() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ["settings"],
    queryFn: getSettings,
  })

  const [formData, setFormData] = useState<Record<string, string>>({})

  const prevSettingsRef = useRef(data?.data?.settings)
  if (data?.data?.settings && data.data.settings !== prevSettingsRef.current) {
    prevSettingsRef.current = data.data.settings
    setFormData(data.data.settings)
  }

  const mutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] })
      toast.success("Settings saved")
    },
  })

  if (isLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>System Configuration</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground mb-4 text-sm">
          Configure system behavior. Changes take effect immediately.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            mutation.mutate(formData)
          }}
          className="flex flex-col gap-4"
        >
          {Object.entries(formData).map(([key, value]) => (
            <div key={key} className="flex flex-col gap-2">
              <Label>{key}</Label>
              <Input
                value={value}
                onChange={(e) => setFormData((prev) => ({ ...prev, [key]: e.target.value }))}
              />
            </div>
          ))}
          {Object.keys(formData).length === 0 && (
            <p className="text-muted-foreground text-sm">No settings configured yet.</p>
          )}
          <Button type="submit" disabled={mutation.isPending}>
            Save Changes
          </Button>
        </form>
      </CardContent>
    </Card>
  )
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
  const [createForm, setCreateForm] = useState<ApiKeyFormData>(EMPTY_FORM)
  const [editingKey, setEditingKey] = useState<ApiKeyRecord | null>(null)
  const [editForm, setEditForm] = useState<ApiKeyFormData>(EMPTY_FORM)

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
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <p className="text-muted-foreground text-sm">
          Manage API keys for accessing the LLM gateway.
        </p>
        <Dialog
          open={showCreate}
          onOpenChange={(open) => {
            setShowCreate(open)
            if (!open) {
              setCreatedKey(null)
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
      </div>

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

      {isLoading ? (
        <p className="text-muted-foreground">Loading...</p>
      ) : (
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
                      <Button
                        variant="ghost"
                        size="icon"
                        onClick={() => {
                          if (confirm("Delete this API key?")) {
                            deleteMutation.mutate(k.id)
                          }
                        }}
                      >
                        <Trash2 className="text-destructive h-4 w-4" />
                      </Button>
                    </div>
                  </TableCell>
                </motion.tr>
              ))}
            </AnimatePresence>
          </TableBody>
        </Table>
      )}
    </div>
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

// ───────────── Backup Section ─────────────

interface ImportResult {
  channels?: { added: number; skipped: number }
  groups?: { added: number; skipped: number }
  groupItems?: { added: number; skipped: number }
  apiKeys?: { added: number; skipped: number }
  settings?: { added: number; skipped: number }
}

function formatFileSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`
}

function BackupSection() {
  const [includeLogs, setIncludeLogs] = useState(false)
  const [includeStats, setIncludeStats] = useState(false)
  const [exporting, setExporting] = useState(false)

  const [selectedFile, setSelectedFile] = useState<File | null>(null)
  const [importing, setImporting] = useState(false)
  const [importResult, setImportResult] = useState<ImportResult | null>(null)
  const [isDragOver, setIsDragOver] = useState(false)
  const fileInputRef = useRef<HTMLInputElement>(null)

  const handleExport = async () => {
    setExporting(true)
    try {
      const resp = await exportData(includeLogs)
      if (!resp.ok) {
        throw new Error("Export failed")
      }
      const blob = await resp.blob()
      const url = URL.createObjectURL(blob)
      const a = document.createElement("a")
      a.href = url
      a.download = `wheel-export-${new Date().toISOString().split("T")[0]}.json`
      document.body.appendChild(a)
      a.click()
      document.body.removeChild(a)
      URL.revokeObjectURL(url)
      toast.success("Export downloaded")
    } catch {
      toast.error("Failed to export data")
    } finally {
      setExporting(false)
    }
  }

  const handleImport = async () => {
    if (!selectedFile) return
    setImporting(true)
    setImportResult(null)
    try {
      const result = await importData(selectedFile)
      if (result.success) {
        setImportResult(result.data as ImportResult)
        toast.success("Import completed")
      } else {
        toast.error(result.error ?? "Import failed")
      }
    } catch {
      toast.error("Failed to import data")
    } finally {
      setImporting(false)
    }
  }

  const handleFileSelect = useCallback((file: File) => {
    if (!file.name.endsWith(".json")) {
      toast.error("Only .json files are supported")
      return
    }
    setSelectedFile(file)
    setImportResult(null)
  }, [])

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOver(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    setIsDragOver(false)
  }, [])

  const handleDrop = useCallback(
    (e: React.DragEvent) => {
      e.preventDefault()
      setIsDragOver(false)
      const file = e.dataTransfer.files[0]
      if (file) handleFileSelect(file)
    },
    [handleFileSelect],
  )

  return (
    <div className="flex flex-col gap-6">
      {/* Export */}
      <Card>
        <CardHeader>
          <CardTitle>Export Data</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-muted-foreground text-sm">
            Export all configuration data as a JSON file.
          </p>

          <div className="flex flex-col gap-3">
            <div className="flex items-center gap-2">
              <Switch id="include-logs" checked={includeLogs} onCheckedChange={setIncludeLogs} />
              <Label htmlFor="include-logs">Include request logs</Label>
            </div>
            <div className="flex items-center gap-2">
              <Switch id="include-stats" checked={includeStats} onCheckedChange={setIncludeStats} />
              <Label htmlFor="include-stats">Include statistics data</Label>
            </div>
          </div>

          <Button onClick={handleExport} disabled={exporting} className="w-fit">
            <Download className="mr-2 h-4 w-4" />
            {exporting ? "Exporting..." : "Download Export"}
          </Button>
        </CardContent>
      </Card>

      {/* Import */}
      <Card>
        <CardHeader>
          <CardTitle>Import Data</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col gap-4">
          <p className="text-muted-foreground text-sm">
            Import configuration from a JSON file. Existing data will not be overwritten.
          </p>

          <div
            className={`relative flex cursor-pointer flex-col items-center justify-center gap-2 rounded-lg border-2 border-dashed p-8 transition-colors ${
              isDragOver
                ? "border-primary bg-primary/5"
                : "border-muted-foreground/25 hover:border-muted-foreground/50"
            }`}
            onClick={() => fileInputRef.current?.click()}
            onDragOver={handleDragOver}
            onDragLeave={handleDragLeave}
            onDrop={handleDrop}
          >
            <FileUp className="text-muted-foreground h-8 w-8" />
            <p className="text-muted-foreground text-sm">Drop file here or click to select</p>
            <p className="text-muted-foreground text-xs">Supports .json files</p>
            <input
              ref={fileInputRef}
              type="file"
              accept=".json"
              className="hidden"
              onChange={(e) => {
                const file = e.target.files?.[0]
                if (file) handleFileSelect(file)
              }}
            />
          </div>

          {selectedFile && (
            <p className="text-muted-foreground text-sm">
              Selected: <span className="text-foreground font-medium">{selectedFile.name}</span> (
              {formatFileSize(selectedFile.size)})
            </p>
          )}

          <Button onClick={handleImport} disabled={!selectedFile || importing} className="w-fit">
            <Upload className="mr-2 h-4 w-4" />
            {importing ? "Importing..." : "Import Data"}
          </Button>

          {importResult && (
            <div className="rounded-md border p-4">
              <p className="mb-2 text-sm font-medium">Import Result:</p>
              <ul className="text-muted-foreground flex flex-col gap-1 text-sm">
                {Object.entries(importResult).map(([key, value]) => (
                  <li key={key}>
                    {key}: {value.added} added, {value.skipped} skipped
                  </li>
                ))}
              </ul>
            </div>
          )}
        </CardContent>
      </Card>
    </div>
  )
}

// ───────────── Settings Page ─────────────

export default function SettingsPage() {
  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-2xl font-bold tracking-tight">Settings</h2>

      <Tabs defaultValue="apikeys">
        <TabsList>
          <TabsTrigger value="apikeys">API Keys</TabsTrigger>
          <TabsTrigger value="account">Account</TabsTrigger>
          <TabsTrigger value="system">System</TabsTrigger>
          <TabsTrigger value="backup">Backup</TabsTrigger>
        </TabsList>

        <TabsContent value="apikeys" className="mt-4">
          <ApiKeysSection />
        </TabsContent>

        <TabsContent value="account" className="mt-4">
          <AccountSection />
        </TabsContent>

        <TabsContent value="system" className="mt-4">
          <SystemConfigSection />
        </TabsContent>

        <TabsContent value="backup" className="mt-4">
          <BackupSection />
        </TabsContent>
      </Tabs>
    </div>
  )
}

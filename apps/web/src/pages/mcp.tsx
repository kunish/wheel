import type { MCPClientInput, MCPClientRecord } from "@/lib/api-client"
import { useQuery } from "@tanstack/react-query"
import { Copy, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
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
import { Card, CardContent } from "@/components/ui/card"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Switch } from "@/components/ui/switch"
import { getApiBaseUrl, listMCPClients } from "@/lib/api-client"
import MCPClientDialog from "./mcp/mcp-client-dialog"
import { useMCPMutations } from "./mcp/use-mcp-mutations"

const EMPTY_FORM: MCPClientInput = {
  name: "",
  connectionType: "http",
  connectionString: "",
  authType: "none",
  headers: [],
  toolsToExecute: [],
  toolsToAutoExec: [],
  enabled: true,
}

export default function MCPPage() {
  const { t } = useTranslation("mcp")

  const { data, isLoading } = useQuery({
    queryKey: ["mcp-clients"],
    queryFn: listMCPClients,
  })
  const clients = data?.data?.clients ?? []

  const [dialogOpen, setDialogOpen] = useState(false)
  const [form, setForm] = useState<MCPClientInput>({ ...EMPTY_FORM })
  const [editingId, setEditingId] = useState<number | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<MCPClientRecord | null>(null)

  const { createMut, updateMut, deleteMut, reconnectMut, toggleMut } = useMCPMutations({
    onSaveSuccess: () => setDialogOpen(false),
  })

  function openCreate() {
    setEditingId(null)
    setForm({ ...EMPTY_FORM, headers: [] })
    setDialogOpen(true)
  }

  function openEdit(client: MCPClientRecord) {
    setEditingId(client.id)
    setForm({
      name: client.name,
      connectionType: client.connectionType,
      connectionString: client.connectionString,
      stdioConfig: client.stdioConfig,
      authType: client.authType,
      headers: client.headers ? [...client.headers] : [],
      toolsToExecute: [...(client.toolsToExecute ?? [])],
      toolsToAutoExec: [...(client.toolsToAutoExec ?? [])],
      enabled: client.enabled,
    })
    setDialogOpen(true)
  }

  function handleSave() {
    if (!form.name.trim()) return
    if (editingId !== null) {
      updateMut.mutate({ ...form, id: editingId })
    } else {
      createMut.mutate(form)
    }
  }

  const isPending = createMut.isPending || updateMut.isPending

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{t("pageTitle")}</h1>
          <p className="text-muted-foreground text-sm">{t("description")}</p>
        </div>
        <Button onClick={openCreate}>
          <Plus className="mr-1 h-4 w-4" />
          {t("addClient")}
        </Button>
      </div>

      {isLoading ? (
        <p className="text-muted-foreground text-sm">{t("actions.loading", { ns: "common" })}</p>
      ) : clients.length === 0 ? (
        <EmptyGuide t={t} onAdd={openCreate} />
      ) : (
        <>
          <EndpointCard />
          <div className="flex flex-col gap-3">
            {clients.map((client) => (
              <ClientCard
                key={client.id}
                client={client}
                onEdit={openEdit}
                onDelete={setDeleteTarget}
                onToggle={(id, enabled) => toggleMut.mutate({ id, enabled })}
                onReconnect={(id) => reconnectMut.mutate(id)}
              />
            ))}
          </div>
        </>
      )}

      <MCPClientDialog
        open={dialogOpen}
        onOpenChange={setDialogOpen}
        form={form}
        setForm={setForm}
        onSave={handleSave}
        isPending={isPending}
        isEdit={editingId !== null}
      />

      <AlertDialog open={deleteTarget !== null} onOpenChange={(o) => !o && setDeleteTarget(null)}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>
              {t("deleteDialog.title", { name: deleteTarget?.name })}
            </AlertDialogTitle>
            <AlertDialogDescription>{t("deleteDialog.description")}</AlertDialogDescription>
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
    </div>
  )
}

// ── Client Card ──

function StateBadge({ state }: { state: MCPClientRecord["state"] }) {
  const { t } = useTranslation("mcp")
  const variant =
    state === "connected" ? "default" : state === "error" ? "destructive" : "secondary"
  return (
    <Badge variant={variant} className="text-[10px]">
      {t(`state.${state}`)}
    </Badge>
  )
}

function ToolsPopover({ tools }: { tools: MCPClientRecord["tools"] }) {
  const { t } = useTranslation("mcp")
  if (!tools || tools.length === 0) {
    return <span className="text-muted-foreground text-xs">{t("tools.noTools")}</span>
  }
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Badge variant="outline" className="cursor-pointer text-[10px]">
          {t("tools.count", { count: tools.length })}
        </Badge>
      </PopoverTrigger>
      <PopoverContent className="max-h-[50vh] w-72 overflow-hidden p-0" align="start" side="bottom">
        <div className="max-h-[50vh] overflow-y-auto p-2">
          <div className="flex flex-col gap-1">
            {tools.map((tool) => (
              <div key={tool.name} className="flex flex-col gap-0.5 rounded px-2 py-1 text-xs">
                <span className="font-mono font-medium">{tool.name}</span>
                {tool.description && (
                  <span className="text-muted-foreground line-clamp-2">{tool.description}</span>
                )}
              </div>
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

function ClientCard({
  client,
  onEdit,
  onDelete,
  onToggle,
  onReconnect,
}: {
  client: MCPClientRecord
  onEdit: (client: MCPClientRecord) => void
  onDelete: (client: MCPClientRecord) => void
  onToggle: (id: number, enabled: boolean) => void
  onReconnect: (id: number) => void
}) {
  const { t } = useTranslation("mcp")
  const endpoint =
    client.connectionType === "stdio"
      ? (client.stdioConfig?.command ?? "—")
      : client.connectionString || "—"

  return (
    <Card>
      <CardContent className="flex flex-col gap-3 p-4">
        {/* Row 1: Name + badges + toggle */}
        <div className="flex items-center justify-between gap-2">
          <div className="flex min-w-0 flex-1 items-center gap-2">
            <span className="truncate font-medium">{client.name}</span>
            <Badge variant="outline" className="shrink-0 text-[10px]">
              {t(`connectionType.${client.connectionType}`)}
            </Badge>
            <StateBadge state={client.state} />
            <ToolsPopover tools={client.tools} />
          </div>
          <Switch
            checked={client.enabled}
            onCheckedChange={(v) => onToggle(client.id, v)}
            aria-label={client.name}
          />
        </div>

        {/* Row 2: Endpoint + actions */}
        <div className="flex items-center justify-between gap-2">
          <span className="text-muted-foreground min-w-0 flex-1 truncate font-mono text-xs">
            {endpoint}
          </span>
          <div className="flex shrink-0 gap-1">
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onReconnect(client.id)}
              title={t("actions.reconnect")}
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onEdit(client)}
              title={t("actions.edit")}
            >
              <Pencil className="h-3.5 w-3.5" />
            </Button>
            <Button
              variant="ghost"
              size="icon"
              className="h-7 w-7"
              onClick={() => onDelete(client)}
              title={t("actions.delete")}
            >
              <Trash2 className="h-3.5 w-3.5" />
            </Button>
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

// ── Empty State Guide ──

function EmptyGuide({ t, onAdd }: { t: (key: string) => string; onAdd: () => void }) {
  return (
    <Card>
      <CardContent className="flex flex-col gap-4 p-5">
        <h2 className="text-lg font-semibold">{t("guide.title")}</h2>

        <div className="flex flex-col gap-3">
          <div className="flex flex-col gap-1">
            <span className="text-sm font-medium">1. {t("guide.step1Title")}</span>
            <span className="text-muted-foreground text-sm">{t("guide.step1Desc")}</span>
            <div className="mt-1 flex flex-col gap-1 pl-4 text-xs">
              <span>
                <span className="font-mono font-medium">HTTP</span> — {t("guide.httpDesc")}
              </span>
              <span>
                <span className="font-mono font-medium">SSE</span> — {t("guide.sseDesc")}
              </span>
              <span>
                <span className="font-mono font-medium">STDIO</span> — {t("guide.stdioDesc")}
              </span>
            </div>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-sm font-medium">2. {t("guide.step2Title")}</span>
            <span className="text-muted-foreground text-sm">{t("guide.step2Desc")}</span>
            <code className="bg-muted mt-1 rounded px-2 py-1 text-xs">{getMCPServerUrl()}</code>
          </div>

          <div className="flex flex-col gap-1">
            <span className="text-sm font-medium">3. {t("guide.step3Title")}</span>
            <span className="text-muted-foreground text-sm">{t("guide.step3Desc")}</span>
            <code className="bg-muted mt-1 rounded px-2 py-1 text-xs">
              POST {getApiBaseUrl() || ""}/v1/mcp/tool/execute
            </code>
          </div>
        </div>

        <Button onClick={onAdd} className="w-fit">
          <Plus className="mr-1 h-4 w-4" />
          {t("addClient")}
        </Button>
      </CardContent>
    </Card>
  )
}

// ── MCP Server Endpoint Card ──

function getMCPServerUrl() {
  const base = getApiBaseUrl() || window.location.origin
  return `${base}/mcp/`
}

function EndpointCard() {
  const { t } = useTranslation("mcp")
  const url = getMCPServerUrl()

  function copyUrl() {
    navigator.clipboard.writeText(url)
    toast.success(t("endpoint.copied"))
  }

  return (
    <Card>
      <CardContent className="flex items-center justify-between gap-3 p-4">
        <div className="flex min-w-0 flex-1 flex-col gap-0.5">
          <span className="text-xs font-medium">{t("endpoint.title")}</span>
          <code className="text-muted-foreground truncate text-xs">{url}</code>
        </div>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 shrink-0"
          onClick={copyUrl}
          title={t("actions.copy", { ns: "common" })}
        >
          <Copy className="h-3.5 w-3.5" />
        </Button>
      </CardContent>
    </Card>
  )
}

import type { MCPClientInput, MCPHeaderEntry, MCPOAuthConfig } from "@/lib/api"
import { Loader2, Plus, Search, X } from "lucide-react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
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
import { discoverOAuthMetadata } from "@/lib/api"

const CONNECTION_TYPES = ["http", "sse", "stdio"] as const
const AUTH_TYPES = ["none", "headers", "oauth"] as const
const HEADER_ROW_KEY = Symbol("headerRowKey")

function getHeaderRowKey(header: MCPHeaderEntry): string {
  const keyedHeader = header as MCPHeaderEntry & { [HEADER_ROW_KEY]?: string }
  if (!keyedHeader[HEADER_ROW_KEY]) keyedHeader[HEADER_ROW_KEY] = crypto.randomUUID()
  return keyedHeader[HEADER_ROW_KEY]
}

/** Returns true if OAuth config is valid (or authType is not oauth). */
function isOAuthValid(form: MCPClientInput): boolean {
  if (form.authType !== "oauth" || form.connectionType === "stdio") return true
  const cfg = form.oauthConfig
  if (!cfg) return false
  // Either a pre-configured access token or (clientId + tokenUrl) must be set
  if (cfg.accessToken) return true
  return !!(cfg.clientId && cfg.tokenUrl)
}

export default function MCPClientDialog({
  open,
  onOpenChange,
  form,
  setForm,
  onSave,
  isPending,
  isEdit,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: MCPClientInput
  setForm: (f: MCPClientInput) => void
  onSave: () => void
  isPending: boolean
  isEdit: boolean
}) {
  const { t } = useTranslation("mcp")

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] w-full max-w-lg overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{isEdit ? t("form.editTitle") : t("form.createTitle")}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          {/* Name + Enabled */}
          <div className="flex items-end gap-3">
            <div className="flex flex-1 flex-col gap-1.5">
              <Label>{t("form.name")}</Label>
              <Input
                value={form.name}
                onChange={(e) => setForm({ ...form, name: e.target.value })}
                placeholder={t("form.namePlaceholder")}
              />
            </div>
            <div className="flex flex-col items-center gap-1.5">
              <Label>{t("form.enabled")}</Label>
              <Switch
                checked={form.enabled}
                onCheckedChange={(v) => setForm({ ...form, enabled: v })}
                className="mt-1"
              />
            </div>
          </div>

          {/* Connection Type */}
          <div className="flex flex-col gap-1.5">
            <Label>{t("form.connectionType")}</Label>
            <Select
              value={form.connectionType}
              onValueChange={(v: MCPClientInput["connectionType"]) =>
                setForm({ ...form, connectionType: v })
              }
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {CONNECTION_TYPES.map((ct) => (
                  <SelectItem key={ct} value={ct}>
                    {t(`connectionType.${ct}`)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          {/* Connection String (HTTP/SSE) */}
          {form.connectionType !== "stdio" && (
            <div className="flex flex-col gap-1.5">
              <Label>{t("form.connectionString")}</Label>
              <Input
                value={form.connectionString ?? ""}
                onChange={(e) => setForm({ ...form, connectionString: e.target.value })}
                placeholder={t("form.connectionStringPlaceholder")}
              />
            </div>
          )}

          {/* STDIO Config */}
          {form.connectionType === "stdio" && <StdioFields form={form} setForm={setForm} t={t} />}

          {/* Auth Type (HTTP/SSE only) */}
          {form.connectionType !== "stdio" && (
            <div className="flex flex-col gap-1.5">
              <Label>{t("form.authType")}</Label>
              <Select
                value={form.authType}
                onValueChange={(v: MCPClientInput["authType"]) =>
                  setForm({
                    ...form,
                    authType: v,
                    headers: v === "headers" ? (form.headers ?? []) : [],
                    oauthConfig: v === "oauth" ? form.oauthConfig : undefined,
                  })
                }
              >
                <SelectTrigger>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {AUTH_TYPES.map((at) => (
                    <SelectItem key={at} value={at}>
                      {t(`authType.${at}`)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          {/* Headers */}
          {form.authType === "headers" && form.connectionType !== "stdio" && (
            <HeadersEditor
              headers={form.headers ?? []}
              onChange={(headers) => setForm({ ...form, headers })}
              t={t}
            />
          )}

          {/* OAuth Config */}
          {form.authType === "oauth" && form.connectionType !== "stdio" && (
            <OAuthFields form={form} setForm={setForm} t={t} />
          )}

          {/* Tool Filters */}
          <div className="flex flex-col gap-1.5">
            <Label>{t("form.toolsToExecute")}</Label>
            <Input
              value={(form.toolsToExecute ?? []).join(", ")}
              onChange={(e) =>
                setForm({
                  ...form,
                  toolsToExecute: e.target.value
                    ? e.target.value.split(",").map((s) => s.trim())
                    : [],
                })
              }
              placeholder={t("form.toolsToExecutePlaceholder")}
            />
          </div>

          <Button
            type="button"
            onClick={onSave}
            disabled={isPending || !form.name || !isOAuthValid(form)}
          >
            {isPending
              ? t("actions.loading", { ns: "common" })
              : t("actions.save", { ns: "common" })}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

// ── STDIO Fields ──

function StdioFields({
  form,
  setForm,
  t,
}: {
  form: MCPClientInput
  setForm: (f: MCPClientInput) => void
  t: (key: string) => string
}) {
  const cfg = form.stdioConfig ?? { command: "", args: [], envs: [] }

  return (
    <div className="flex flex-col gap-3 rounded-md border p-3">
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.stdioCommand")}</Label>
        <Input
          value={cfg.command}
          onChange={(e) => setForm({ ...form, stdioConfig: { ...cfg, command: e.target.value } })}
          placeholder={t("form.stdioCommandPlaceholder")}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.stdioArgs")}</Label>
        <textarea
          className="border-input bg-background placeholder:text-muted-foreground min-h-[60px] w-full rounded-md border px-3 py-2 text-sm"
          value={(cfg.args ?? []).join("\n")}
          onChange={(e) =>
            setForm({
              ...form,
              stdioConfig: {
                ...cfg,
                args: e.target.value ? e.target.value.split("\n") : [],
              },
            })
          }
          placeholder={t("form.stdioArgsPlaceholder")}
        />
      </div>
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.stdioEnvs")}</Label>
        <textarea
          className="border-input bg-background placeholder:text-muted-foreground min-h-[60px] w-full rounded-md border px-3 py-2 text-sm"
          value={(cfg.envs ?? []).join("\n")}
          onChange={(e) =>
            setForm({
              ...form,
              stdioConfig: {
                ...cfg,
                envs: e.target.value ? e.target.value.split("\n") : [],
              },
            })
          }
          placeholder={t("form.stdioEnvsPlaceholder")}
        />
      </div>
    </div>
  )
}

// ── Headers Editor ──

function HeadersEditor({
  headers,
  onChange,
  t,
}: {
  headers: MCPHeaderEntry[]
  onChange: (headers: MCPHeaderEntry[]) => void
  t: (key: string) => string
}) {
  function update(idx: number, patch: Partial<MCPHeaderEntry>) {
    const next = [...headers]
    next[idx] = { ...next[idx], ...patch }
    onChange(next)
  }

  function remove(idx: number) {
    onChange(headers.filter((_, i) => i !== idx))
  }

  function add() {
    onChange([...headers, { key: "", value: "" }])
  }

  return (
    <div className="flex flex-col gap-2">
      <Label>{t("form.headers")}</Label>
      {headers.map((h, idx) => (
        <div key={getHeaderRowKey(h)} className="flex items-center gap-2">
          <Input
            className="flex-1"
            placeholder={t("form.headerKey")}
            value={h.key}
            onChange={(e) => update(idx, { key: e.target.value })}
          />
          <Input
            className="flex-1"
            placeholder={t("form.headerValue")}
            value={h.value}
            onChange={(e) => update(idx, { value: e.target.value })}
          />
          <Button
            type="button"
            variant="ghost"
            size="icon"
            className="h-8 w-8 shrink-0"
            onClick={() => remove(idx)}
          >
            <X className="h-4 w-4" />
          </Button>
        </div>
      ))}
      <Button type="button" variant="outline" size="sm" className="w-fit" onClick={add}>
        <Plus className="mr-1 h-3.5 w-3.5" />
        {t("form.addHeader")}
      </Button>
    </div>
  )
}

// ── OAuth Fields ──

function OAuthFields({
  form,
  setForm,
  t,
}: {
  form: MCPClientInput
  setForm: (f: MCPClientInput) => void
  t: (key: string) => string
}) {
  const [discovering, setDiscovering] = useState(false)
  const cfg: MCPOAuthConfig = form.oauthConfig ?? {
    clientId: "",
    tokenUrl: "",
  }

  function update(patch: Partial<MCPOAuthConfig>) {
    setForm({ ...form, oauthConfig: { ...cfg, ...patch } })
  }

  async function handleDiscover() {
    const url = form.connectionString
    if (!url) return
    setDiscovering(true)
    try {
      const res = await discoverOAuthMetadata(url)
      if (res.success && res.data) {
        update({
          tokenUrl: res.data.tokenUrl,
          authorizationUrl: res.data.authorizationUrl,
          scopes: res.data.scopes?.join(" ") || "",
        })
      }
    } catch {
      // discovery is best-effort, fields can be filled manually
    } finally {
      setDiscovering(false)
    }
  }

  return (
    <div className="flex flex-col gap-3 rounded-md border p-3">
      {/* Auto Discover */}
      <Button
        type="button"
        variant="outline"
        size="sm"
        className="w-fit"
        disabled={discovering || !form.connectionString}
        onClick={handleDiscover}
      >
        {discovering ? (
          <Loader2 className="mr-1 h-3.5 w-3.5 animate-spin" />
        ) : (
          <Search className="mr-1 h-3.5 w-3.5" />
        )}
        {t("form.oauthDiscoverButton")}
      </Button>

      {/* Client ID */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.oauthClientId")}</Label>
        <Input
          value={cfg.clientId}
          onChange={(e) => update({ clientId: e.target.value })}
          placeholder={t("form.oauthClientIdPlaceholder")}
        />
      </div>

      {/* Client Secret */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.oauthClientSecret")}</Label>
        <Input
          type="password"
          value={cfg.clientSecret ?? ""}
          onChange={(e) => update({ clientSecret: e.target.value })}
          placeholder={t("form.oauthClientSecretPlaceholder")}
        />
      </div>

      {/* Token URL */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.oauthTokenUrl")}</Label>
        <Input
          value={cfg.tokenUrl}
          onChange={(e) => update({ tokenUrl: e.target.value })}
          placeholder={t("form.oauthTokenUrlPlaceholder")}
        />
      </div>

      {/* Scopes */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.oauthScopes")}</Label>
        <Input
          value={cfg.scopes ?? ""}
          onChange={(e) => update({ scopes: e.target.value })}
          placeholder={t("form.oauthScopesPlaceholder")}
        />
      </div>

      {/* Divider */}
      <div className="text-muted-foreground flex items-center gap-2 text-xs">
        <div className="bg-border h-px flex-1" />
        {t("form.oauthOrDivider")}
        <div className="bg-border h-px flex-1" />
      </div>

      {/* Pre-configured Access Token */}
      <div className="flex flex-col gap-1.5">
        <Label className="text-xs">{t("form.oauthAccessToken")}</Label>
        <Input
          type="password"
          value={cfg.accessToken ?? ""}
          onChange={(e) => update({ accessToken: e.target.value })}
          placeholder={t("form.oauthAccessTokenPlaceholder")}
        />
      </div>
    </div>
  )
}

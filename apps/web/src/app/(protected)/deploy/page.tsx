"use client"

import { Cloud, Copy, Download, Globe } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"

type DeployTarget = "cloudflare" | "vercel" | null

interface DeployConfig {
  workerName: string
  d1DatabaseName: string
  kvNamespace: string
  jwtSecret: string
  adminUsername: string
  adminPassword: string
}

const DEFAULT_CONFIG: DeployConfig = {
  workerName: "wheel",
  d1DatabaseName: "wheel-db",
  kvNamespace: "wheel-cache",
  jwtSecret: "",
  adminUsername: "admin",
  adminPassword: "",
}

const STEPS = ["Target", "Configure", "Preview", "Deploy"]

export default function DeployPage() {
  const [step, setStep] = useState(0)
  const [target, setTarget] = useState<DeployTarget>(null)
  const [config, setConfig] = useState<DeployConfig>(DEFAULT_CONFIG)

  function renderStep() {
    switch (step) {
      case 0:
        return <TargetStep target={target} onSelect={setTarget} />
      case 1:
        return <ConfigureStep config={config} onChange={setConfig} />
      case 2:
        return <PreviewStep target={target!} config={config} />
      case 3:
        return <DeployStep target={target!} config={config} />
      default:
        return null
    }
  }

  return (
    <div className="flex flex-col gap-6">
      <h2 className="text-2xl font-bold tracking-tight">Deploy Wizard</h2>

      <div className="flex items-center gap-2">
        {STEPS.map((label, i) => (
          <div key={label} className="flex items-center gap-2">
            <Badge variant={i <= step ? "default" : "outline"}>{i + 1}</Badge>
            <span className={`text-sm ${i <= step ? "font-medium" : "text-muted-foreground"}`}>
              {label}
            </span>
            {i < STEPS.length - 1 && <div className="bg-border mx-2 h-px w-8" />}
          </div>
        ))}
      </div>

      <div className="max-w-2xl">{renderStep()}</div>

      <div className="flex max-w-2xl gap-3">
        {step > 0 && (
          <Button variant="outline" onClick={() => setStep((s) => s - 1)}>
            Back
          </Button>
        )}
        {step < STEPS.length - 1 && (
          <Button onClick={() => setStep((s) => s + 1)} disabled={step === 0 && !target}>
            Next
          </Button>
        )}
      </div>
    </div>
  )
}

function TargetStep({
  target,
  onSelect,
}: {
  target: DeployTarget
  onSelect: (t: DeployTarget) => void
}) {
  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card
        className={`cursor-pointer transition-colors ${target === "cloudflare" ? "border-primary" : ""}`}
        onClick={() => onSelect("cloudflare")}
      >
        <CardHeader>
          <Cloud className="mb-2 h-8 w-8" />
          <CardTitle>Cloudflare Worker</CardTitle>
          <CardDescription>
            Deploy the API proxy to Cloudflare Workers with D1 database and KV storage.
          </CardDescription>
        </CardHeader>
      </Card>
      <Card
        className={`cursor-pointer transition-colors ${target === "vercel" ? "border-primary" : ""}`}
        onClick={() => onSelect("vercel")}
      >
        <CardHeader>
          <Globe className="mb-2 h-8 w-8" />
          <CardTitle>Vercel</CardTitle>
          <CardDescription>
            Deploy the management dashboard to Vercel with automatic builds.
          </CardDescription>
        </CardHeader>
      </Card>
    </div>
  )
}

function ConfigureStep({
  config,
  onChange,
}: {
  config: DeployConfig
  onChange: (c: DeployConfig) => void
}) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Configuration</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {(
          [
            ["workerName", "Worker Name"],
            ["d1DatabaseName", "D1 Database Name"],
            ["kvNamespace", "KV Namespace"],
            ["jwtSecret", "JWT Secret"],
            ["adminUsername", "Admin Username"],
            ["adminPassword", "Admin Password"],
          ] as const
        ).map(([key, label]) => (
          <div key={key} className="flex flex-col gap-2">
            <Label>{label}</Label>
            <Input
              value={config[key]}
              onChange={(e) => onChange({ ...config, [key]: e.target.value })}
              type={key.includes("assword") || key.includes("ecret") ? "password" : "text"}
            />
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

function PreviewStep({ target, config }: { target: DeployTarget; config: DeployConfig }) {
  const files =
    target === "cloudflare"
      ? [
          { name: "wrangler.toml", content: generateWranglerToml(config) },
          { name: ".env.example", content: generateEnvExample(config) },
          { name: "init.sql", content: generateD1MigrationSQL() },
        ]
      : [
          { name: "vercel.json", content: generateVercelJson(config) },
          { name: ".env.example", content: generateEnvExample(config) },
        ]

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle>Configuration Files</CardTitle>
        <Button
          variant="outline"
          size="sm"
          onClick={() =>
            downloadZip(files, target === "cloudflare" ? "wheel-cf-config" : "wheel-vercel-config")
          }
        >
          <Download className="mr-2 h-4 w-4" /> Download ZIP
        </Button>
      </CardHeader>
      <CardContent>
        <Tabs defaultValue={files[0].name}>
          <TabsList>
            {files.map((f) => (
              <TabsTrigger key={f.name} value={f.name}>
                {f.name}
              </TabsTrigger>
            ))}
          </TabsList>
          {files.map((f) => (
            <TabsContent key={f.name} value={f.name}>
              <div className="relative">
                <Button
                  variant="ghost"
                  size="icon"
                  className="absolute top-2 right-2"
                  onClick={() => {
                    navigator.clipboard.writeText(f.content)
                    toast.success(`${f.name} copied!`)
                  }}
                >
                  <Copy className="h-4 w-4" />
                </Button>
                <pre className="bg-muted overflow-x-auto rounded-md p-4 text-sm">{f.content}</pre>
              </div>
            </TabsContent>
          ))}
        </Tabs>
      </CardContent>
    </Card>
  )
}

function DeployStep({ target, config }: { target: DeployTarget; config: DeployConfig }) {
  const commands =
    target === "cloudflare"
      ? [
          `npx wrangler d1 create ${config.d1DatabaseName}`,
          `npx wrangler kv namespace create ${config.kvNamespace}`,
          `npx wrangler d1 migrations apply ${config.d1DatabaseName} --local`,
          `npx wrangler secret put JWT_SECRET`,
          `npx wrangler deploy`,
        ]
      : [
          `npx vercel link`,
          `npx vercel env add NEXT_PUBLIC_API_BASE_URL`,
          `npx vercel deploy --prod`,
        ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Deploy Commands</CardTitle>
        <CardDescription>Run these commands in your terminal to deploy.</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {commands.map((cmd) => (
          <div key={cmd} className="bg-muted flex items-center justify-between rounded-md p-3">
            <code className="text-sm">{cmd}</code>
            <Button
              variant="ghost"
              size="icon"
              onClick={() => {
                navigator.clipboard.writeText(cmd)
                toast.success("Copied!")
              }}
            >
              <Copy className="h-4 w-4" />
            </Button>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

// --- Config generators ---

function generateWranglerToml(config: DeployConfig): string {
  return `name = "${config.workerName}"
main = "src/index.ts"
compatibility_date = "2025-02-07"

[[d1_databases]]
binding = "DB"
database_name = "${config.d1DatabaseName}"
database_id = "<YOUR_D1_DATABASE_ID>"

[[kv_namespaces]]
binding = "CACHE"
id = "<YOUR_KV_NAMESPACE_ID>"

[vars]
ADMIN_USERNAME = "${config.adminUsername}"
ADMIN_PASSWORD = "${config.adminPassword}"
`
}

function generateVercelJson(config: DeployConfig): string {
  return JSON.stringify(
    {
      buildCommand: "pnpm --filter @wheel/web build",
      outputDirectory: "apps/web/.next",
      installCommand: "pnpm install",
      framework: "nextjs",
      env: {
        NEXT_PUBLIC_API_BASE_URL: `https://${config.workerName}.<your-subdomain>.workers.dev`,
      },
    },
    null,
    2,
  )
}

function generateEnvExample(config: DeployConfig): string {
  return `# ===========================
# Wheel Configuration
# ===========================

# --- Cloudflare Worker ---
# Worker name (used in wrangler.toml)
WORKER_NAME=${config.workerName}

# D1 Database
D1_DATABASE_NAME=${config.d1DatabaseName}
# D1_DATABASE_ID=<from "npx wrangler d1 create">

# KV Namespace
KV_NAMESPACE=${config.kvNamespace}
# KV_NAMESPACE_ID=<from "npx wrangler kv namespace create">

# --- Authentication ---
# JWT secret for admin panel authentication (required)
JWT_SECRET=${config.jwtSecret || "<generate-a-strong-secret>"}

# Default admin credentials (set on first deploy)
ADMIN_USERNAME=${config.adminUsername}
ADMIN_PASSWORD=${config.adminPassword || "<set-a-strong-password>"}

# --- Frontend (Next.js) ---
# URL of the Cloudflare Worker API
NEXT_PUBLIC_API_BASE_URL=https://${config.workerName}.<your-subdomain>.workers.dev
`
}

function generateD1MigrationSQL(): string {
  return `-- Wheel D1 Initial Migration
-- Run: npx wrangler d1 execute <DB_NAME> --file=./init.sql

CREATE TABLE IF NOT EXISTS users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  created_at INTEGER NOT NULL DEFAULT (unixepoch()),
  updated_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS channels (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL UNIQUE,
  type INTEGER NOT NULL DEFAULT 1,
  enabled INTEGER NOT NULL DEFAULT 1,
  base_urls TEXT NOT NULL DEFAULT '[]',
  model TEXT NOT NULL DEFAULT '',
  custom_model TEXT NOT NULL DEFAULT '',
  proxy INTEGER NOT NULL DEFAULT 0,
  auto_sync INTEGER NOT NULL DEFAULT 0,
  auto_group INTEGER NOT NULL DEFAULT 0,
  custom_header TEXT NOT NULL DEFAULT '[]',
  param_override TEXT,
  channel_proxy TEXT,
  match_regex TEXT
);

CREATE TABLE IF NOT EXISTS channel_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  enabled INTEGER NOT NULL DEFAULT 1,
  channel_key TEXT NOT NULL,
  status_code INTEGER NOT NULL DEFAULT 0,
  last_use_timestamp INTEGER NOT NULL DEFAULT 0,
  total_cost REAL NOT NULL DEFAULT 0,
  remark TEXT NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS groups (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  mode INTEGER NOT NULL DEFAULT 1,
  match_regex TEXT NOT NULL DEFAULT '',
  first_token_time_out INTEGER NOT NULL DEFAULT 30
);

CREATE TABLE IF NOT EXISTS group_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  group_id INTEGER NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
  channel_id INTEGER NOT NULL REFERENCES channels(id) ON DELETE CASCADE,
  model_name TEXT NOT NULL DEFAULT '',
  priority INTEGER NOT NULL DEFAULT 0,
  weight INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS api_keys (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  key TEXT NOT NULL UNIQUE,
  enabled INTEGER NOT NULL DEFAULT 1,
  expire_at INTEGER NOT NULL DEFAULT 0,
  cost_limit REAL NOT NULL DEFAULT 0,
  total_cost REAL NOT NULL DEFAULT 0,
  model_whitelist TEXT NOT NULL DEFAULT '',
  created_at INTEGER NOT NULL DEFAULT (unixepoch())
);

CREATE TABLE IF NOT EXISTS relay_logs (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  time INTEGER NOT NULL DEFAULT (unixepoch()),
  api_key_id INTEGER NOT NULL DEFAULT 0,
  api_key_name TEXT NOT NULL DEFAULT '',
  request_model_name TEXT NOT NULL DEFAULT '',
  target_model_name TEXT NOT NULL DEFAULT '',
  channel_id INTEGER NOT NULL DEFAULT 0,
  channel_name TEXT NOT NULL DEFAULT '',
  group_id INTEGER NOT NULL DEFAULT 0,
  group_name TEXT NOT NULL DEFAULT '',
  input_tokens INTEGER NOT NULL DEFAULT 0,
  output_tokens INTEGER NOT NULL DEFAULT 0,
  total_cost REAL NOT NULL DEFAULT 0,
  use_time INTEGER NOT NULL DEFAULT 0,
  stream INTEGER NOT NULL DEFAULT 0,
  error TEXT NOT NULL DEFAULT '',
  attempts TEXT NOT NULL DEFAULT '[]'
);

CREATE TABLE IF NOT EXISTS settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL DEFAULT ''
);

-- Default admin user (password: admin)
-- You should change this password after first login
INSERT OR IGNORE INTO users (username, password) VALUES ('admin', 'admin');
`
}

// --- ZIP download (pure browser, no library needed) ---

async function downloadZip(files: { name: string; content: string }[], zipName: string) {
  // Use a simple approach: create individual file downloads
  // For a proper ZIP, we use the lightweight approach with Blob
  // Since JSZip isn't installed, we offer individual downloads or use a basic ZIP implementation

  if (files.length === 1) {
    downloadFile(files[0].name, files[0].content)
    return
  }

  // Create a simple ZIP using the browser's compression API if available
  // Fallback: download all files individually
  try {
    const { default: JSZip } = await import("jszip")
    const zip = new JSZip()
    for (const f of files) {
      zip.file(f.name, f.content)
    }
    const blob = await zip.generateAsync({ type: "blob" })
    const url = URL.createObjectURL(blob)
    const a = document.createElement("a")
    a.href = url
    a.download = `${zipName}.zip`
    a.click()
    URL.revokeObjectURL(url)
    toast.success("ZIP downloaded!")
  } catch {
    // Fallback: download files individually
    for (const f of files) {
      downloadFile(f.name, f.content)
    }
    toast.success("Files downloaded!")
  }
}

function downloadFile(name: string, content: string) {
  const blob = new Blob([content], { type: "text/plain" })
  const url = URL.createObjectURL(blob)
  const a = document.createElement("a")
  a.href = url
  a.download = name
  a.click()
  URL.revokeObjectURL(url)
}

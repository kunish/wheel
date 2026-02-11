import { Download, Eye, EyeOff, Loader2 } from "lucide-react"
import { useState } from "react"
import { toast } from "sonner"
import { GroupedModelList } from "@/components/grouped-model-list"
import { ModelCard } from "@/components/model-card"
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
import { Textarea } from "@/components/ui/textarea"
import { fetchChannelModelsPreview } from "@/lib/api"

const TYPE_LABELS: Record<number, string> = {
  0: "OpenAI Chat",
  1: "OpenAI",
  2: "Anthropic",
  3: "Gemini",
  4: "Volcengine",
  5: "OpenAI Embedding",
}

export interface ChannelFormData {
  id?: number
  name: string
  type: number
  enabled: boolean
  baseUrls: { url: string; delay: number }[]
  keys: { channelKey: string; remark: string }[]
  model: string[]
  customModel: string
  paramOverride: string
}

export const EMPTY_CHANNEL_FORM: ChannelFormData = {
  name: "",
  type: 1,
  enabled: true,
  baseUrls: [{ url: "", delay: 0 }],
  keys: [{ channelKey: "", remark: "" }],
  model: [],
  customModel: "",
  paramOverride: "",
}

// ─── Model Tag Input ────────────────────────────

function ModelTagInput({
  value,
  onChange,
}: {
  value: string[]
  onChange: (value: string[]) => void
}) {
  const [input, setInput] = useState("")
  const tags = value ?? []

  function addTags(raw: string) {
    const newTags = raw
      .split(/[,\n]/)
      .map((t) => t.trim())
      .filter(Boolean)
    if (newTags.length === 0) return
    const merged = [...new Set([...tags, ...newTags])]
    onChange(merged)
    setInput("")
  }

  function removeTag(tag: string) {
    onChange(tags.filter((t) => t !== tag))
  }

  function handleKeyDown(e: React.KeyboardEvent<HTMLInputElement>) {
    if (e.key === "Enter" || e.key === ",") {
      e.preventDefault()
      addTags(input)
    }
    if (e.key === "Backspace" && input === "" && tags.length > 0) {
      removeTag(tags[tags.length - 1])
    }
  }

  function handlePaste(e: React.ClipboardEvent<HTMLInputElement>) {
    e.preventDefault()
    addTags(e.clipboardData.getData("text"))
  }

  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-2">
        <Input
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={handleKeyDown}
          onPaste={handlePaste}
          onBlur={() => {
            if (input.trim()) addTags(input)
          }}
          placeholder="Type model name, press Enter to add"
          className="flex-1"
        />
        <span className="text-muted-foreground text-xs whitespace-nowrap">
          {tags.length} model{tags.length !== 1 ? "s" : ""}
        </span>
      </div>
      {tags.length > 0 && (
        <div className="max-h-40 overflow-y-auto">
          <GroupedModelList
            models={tags}
            renderModel={(tag) => (
              <ModelCard key={tag} modelId={tag} onRemove={() => removeTag(tag)} />
            )}
          />
        </div>
      )}
    </div>
  )
}

// ─── Fetch Models Button ────────────────────────

function FetchModelsButton({
  form,
  setForm,
}: {
  form: ChannelFormData
  setForm: (f: ChannelFormData) => void
}) {
  const [loading, setLoading] = useState(false)

  const baseUrl = form.baseUrls[0]?.url?.trim()
  const key = form.keys[0]?.channelKey?.trim()
  const canFetch = !!baseUrl && !!key

  async function handleFetch() {
    if (!canFetch) {
      toast.error("Please fill in Base URL and API Key first")
      return
    }
    setLoading(true)
    try {
      const res = await fetchChannelModelsPreview({
        type: form.type,
        baseUrl,
        key,
      })
      const models = res.data.models
      if (models.length === 0) {
        toast.info("No models found from this provider")
        return
      }
      setForm({ ...form, model: models })
      toast.success(`Fetched ${models.length} models`)
    } catch (err) {
      toast.error(err instanceof Error ? err.message : "Failed to fetch models")
    } finally {
      setLoading(false)
    }
  }

  return (
    <Button variant="outline" size="sm" onClick={handleFetch} disabled={!canFetch || loading}>
      {loading ? (
        <Loader2 className="mr-1 h-3 w-3 animate-spin" />
      ) : (
        <Download className="mr-1 h-3 w-3" />
      )}
      {loading ? "Fetching..." : "Fetch Models"}
    </Button>
  )
}

// ─── Channel Dialog ────────────────────────────

export default function ChannelDialog({
  open,
  onOpenChange,
  form,
  setForm,
  onSave,
  isPending,
}: {
  open: boolean
  onOpenChange: (open: boolean) => void
  form: ChannelFormData
  setForm: (f: ChannelFormData) => void
  onSave: () => void
  isPending: boolean
}) {
  const [showKey, setShowKey] = useState(false)

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent className="max-h-[85vh] w-full max-w-2xl max-w-[95vw] overflow-y-auto">
        <DialogHeader>
          <DialogTitle>{form.id ? "Edit Channel" : "Create Channel"}</DialogTitle>
        </DialogHeader>
        <div className="flex flex-col gap-4 py-2">
          <div className="flex flex-col gap-2">
            <Label>Name</Label>
            <Input value={form.name} onChange={(e) => setForm({ ...form, name: e.target.value })} />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Provider Type</Label>
            <Select
              value={String(form.type)}
              onValueChange={(v) => setForm({ ...form, type: Number(v) })}
            >
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                {Object.entries(TYPE_LABELS).map(([val, label]) => (
                  <SelectItem key={val} value={val}>
                    {label}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>Base URL</Label>
            <Input
              placeholder="https://api.openai.com"
              value={form.baseUrls[0]?.url ?? ""}
              onChange={(e) =>
                setForm({
                  ...form,
                  baseUrls: [{ url: e.target.value, delay: form.baseUrls[0]?.delay ?? 0 }],
                })
              }
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>API Key</Label>
            <div className="relative">
              <Input
                type={showKey ? "text" : "password"}
                placeholder="sk-..."
                value={form.keys[0]?.channelKey ?? ""}
                onChange={(e) =>
                  setForm({
                    ...form,
                    keys: [{ channelKey: e.target.value, remark: form.keys[0]?.remark ?? "" }],
                  })
                }
                className="pr-9"
              />
              <Button
                type="button"
                variant="ghost"
                size="icon"
                className="absolute top-1/2 right-1 h-7 w-7 -translate-y-1/2"
                onClick={() => setShowKey(!showKey)}
                aria-label={showKey ? "Hide API key" : "Show API key"}
              >
                {showKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
              </Button>
            </div>
          </div>

          <div className="flex flex-col gap-2">
            <div className="flex items-center justify-between">
              <Label>Models</Label>
              <FetchModelsButton form={form} setForm={setForm} />
            </div>
            <ModelTagInput value={form.model} onChange={(model) => setForm({ ...form, model })} />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Custom Models</Label>
            <Input
              value={form.customModel}
              onChange={(e) => setForm({ ...form, customModel: e.target.value })}
              placeholder="model-alias:actual-model, ..."
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>Parameter Override (JSON)</Label>
            <Textarea
              value={form.paramOverride}
              onChange={(e) => setForm({ ...form, paramOverride: e.target.value })}
              placeholder='{"temperature": 0.7}'
              rows={3}
            />
          </div>

          <Button className="mt-2" onClick={onSave} disabled={isPending || !form.name}>
            {isPending ? "Saving..." : "Save"}
          </Button>
        </div>
      </DialogContent>
    </Dialog>
  )
}

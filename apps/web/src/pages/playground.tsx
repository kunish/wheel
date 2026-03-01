import { useQuery } from "@tanstack/react-query"
import {
  Bot,
  ChevronDown,
  Copy,
  Loader2,
  RotateCcw,
  Send,
  Settings2,
  Square,
  Terminal,
} from "lucide-react"
import { Suspense, useCallback, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from "@/components/ui/collapsible"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Slider } from "@/components/ui/slider"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { getApiBaseUrl, listApiKeys, listGroups } from "@/lib/api-client"
import { LazyMarkdown } from "@/pages/logs/detail/code-block"

interface MessageStats {
  latencyMs: number
  firstTokenMs?: number
  inputTokens?: number
  outputTokens?: number
  totalTokens?: number
  cost?: number
}

export default function PlaygroundPage() {
  const { t } = useTranslation("playground")

  // Form state
  const [model, setModel] = useState("")
  const [systemPrompt, setSystemPrompt] = useState("")
  const [userMessage, setUserMessage] = useState("")
  const [customApiKey, setCustomApiKey] = useState("")
  const [stream, setStream] = useState(true)
  const [temperature, setTemperature] = useState(0.7)
  const [maxTokens, setMaxTokens] = useState(4096)
  const [topP, setTopP] = useState(1)

  // Response state
  const [response, setResponse] = useState("")
  const [isLoading, setIsLoading] = useState(false)
  const [stats, setStats] = useState<MessageStats | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [showParams, setShowParams] = useState(false)

  const abortRef = useRef<AbortController | null>(null)

  // Fetch configured groups as available models
  const { data: groupData } = useQuery({
    queryKey: ["groups"],
    queryFn: () => listGroups(),
  })
  const models = ((groupData?.data?.groups ?? []) as { name: string }[]).map((g) => g.name).sort()

  // Fetch available API keys for /v1/ endpoint auth
  const { data: apiKeyData } = useQuery({
    queryKey: ["apikeys"],
    queryFn: listApiKeys,
  })
  const defaultApiKey = apiKeyData?.data?.apiKeys?.find((k) => k.enabled)?.apiKey ?? ""

  const handleSend = useCallback(async () => {
    if (!model || !userMessage.trim()) return

    setIsLoading(true)
    setResponse("")
    setError(null)
    setStats(null)

    const controller = new AbortController()
    abortRef.current = controller

    const messages = [
      ...(systemPrompt ? [{ role: "system" as const, content: systemPrompt }] : []),
      { role: "user" as const, content: userMessage },
    ]

    const body = {
      model,
      messages,
      stream,
      temperature,
      max_tokens: maxTokens,
      top_p: topP,
    }

    const startTime = performance.now()
    let firstTokenTime: number | undefined

    try {
      const authKey = customApiKey || defaultApiKey || ""
      const baseUrl = getApiBaseUrl()
      const url = baseUrl ? `${baseUrl}/v1/chat/completions` : "/v1/chat/completions"

      const resp = await fetch(url, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          Authorization: `Bearer ${authKey}`,
        },
        body: JSON.stringify(body),
        signal: controller.signal,
      })

      if (!resp.ok) {
        const errBody = await resp.json().catch(() => ({}))
        throw new Error(
          (errBody as { error?: { message?: string } })?.error?.message || `HTTP ${resp.status}`,
        )
      }

      if (stream && resp.body) {
        const reader = resp.body.getReader()
        const decoder = new TextDecoder()
        let fullText = ""
        let usage:
          | { prompt_tokens?: number; completion_tokens?: number; total_tokens?: number }
          | undefined

        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value, { stream: true })
          const lines = chunk.split("\n")

          for (const line of lines) {
            if (!line.startsWith("data: ") || line === "data: [DONE]") continue
            try {
              const json = JSON.parse(line.slice(6))
              const delta = json.choices?.[0]?.delta?.content
              if (delta) {
                if (!firstTokenTime) firstTokenTime = performance.now() - startTime
                fullText += delta
                setResponse(fullText)
              }
              if (json.usage) usage = json.usage
            } catch {
              // skip invalid JSON
            }
          }
        }

        setStats({
          latencyMs: performance.now() - startTime,
          firstTokenMs: firstTokenTime,
          inputTokens: usage?.prompt_tokens,
          outputTokens: usage?.completion_tokens,
          totalTokens: usage?.total_tokens,
        })
      } else {
        const json = await resp.json()
        const content = json.choices?.[0]?.message?.content || ""
        setResponse(content)
        setStats({
          latencyMs: performance.now() - startTime,
          inputTokens: json.usage?.prompt_tokens,
          outputTokens: json.usage?.completion_tokens,
          totalTokens: json.usage?.total_tokens,
        })
      }
    } catch (err: unknown) {
      if ((err as Error).name !== "AbortError") {
        setError((err as Error).message || "Unknown error")
      }
    } finally {
      setIsLoading(false)
      abortRef.current = null
    }
  }, [
    model,
    userMessage,
    systemPrompt,
    stream,
    temperature,
    maxTokens,
    topP,
    customApiKey,
    defaultApiKey,
  ])

  const handleStop = useCallback(() => {
    abortRef.current?.abort()
    setIsLoading(false)
  }, [])

  const handleClear = useCallback(() => {
    setResponse("")
    setError(null)
    setStats(null)
  }, [])

  const handleCopyAsCurl = useCallback(() => {
    const authKey = customApiKey || defaultApiKey || "YOUR_API_KEY"
    const baseUrl = getApiBaseUrl() || window.location.origin
    const messages = [
      ...(systemPrompt ? [{ role: "system", content: systemPrompt }] : []),
      { role: "user", content: userMessage },
    ]
    const body = { model, messages, stream, temperature, max_tokens: maxTokens, top_p: topP }
    const curl = `curl ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${authKey}" \\
  -d '${JSON.stringify(body, null, 2)}'`
    navigator.clipboard.writeText(curl)
    toast.success(t("copiedCurl"))
  }, [
    model,
    userMessage,
    systemPrompt,
    stream,
    temperature,
    maxTokens,
    topP,
    customApiKey,
    defaultApiKey,
    t,
  ])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="grid min-h-0 flex-1 gap-6 lg:grid-cols-2">
        {/* Left: Input */}
        <div className="flex flex-col gap-4 overflow-auto p-1">
          {/* Model selector */}
          <div className="flex flex-col gap-2">
            <Label>{t("model")}</Label>
            <Select value={model} onValueChange={setModel}>
              <SelectTrigger>
                <SelectValue placeholder={t("selectModel")} />
              </SelectTrigger>
              <SelectContent>
                {models.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
                {models.length === 0 && (
                  <div className="text-muted-foreground px-2 py-4 text-center text-sm">
                    {t("noModels")}
                  </div>
                )}
              </SelectContent>
            </Select>
          </div>

          {/* System prompt */}
          <div className="flex flex-col gap-2">
            <Label>{t("systemPrompt")}</Label>
            <Textarea
              value={systemPrompt}
              onChange={(e) => setSystemPrompt(e.target.value)}
              placeholder={t("systemPromptPlaceholder")}
              rows={3}
              className="resize-none"
            />
          </div>

          {/* User message */}
          <div className="flex flex-col gap-2">
            <Label>{t("userMessage")}</Label>
            <Textarea
              value={userMessage}
              onChange={(e) => setUserMessage(e.target.value)}
              placeholder={t("userMessagePlaceholder")}
              rows={6}
              className="min-h-[120px] resize-none"
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault()
                  handleSend()
                }
              }}
            />
          </div>

          {/* Parameters collapsible */}
          <Collapsible open={showParams} onOpenChange={setShowParams}>
            <CollapsibleTrigger asChild>
              <Button variant="ghost" size="sm" className="w-full justify-between">
                <span className="flex items-center gap-2">
                  <Settings2 className="h-4 w-4" />
                  {t("parameters")}
                </span>
                <ChevronDown
                  className={`h-4 w-4 transition-transform ${showParams ? "rotate-180" : ""}`}
                />
              </Button>
            </CollapsibleTrigger>
            <CollapsibleContent className="space-y-4 pt-2">
              {/* API Key */}
              <div className="flex flex-col gap-2">
                <Label>{t("apiKey")}</Label>
                <Input
                  type="password"
                  value={customApiKey}
                  onChange={(e) => setCustomApiKey(e.target.value)}
                  placeholder={t("apiKeyPlaceholder")}
                />
              </div>

              {/* Stream toggle */}
              <div className="flex items-center gap-2">
                <Switch id="stream-toggle" checked={stream} onCheckedChange={setStream} />
                <Label htmlFor="stream-toggle">{t("stream")}</Label>
              </div>

              {/* Temperature */}
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("temperature")}</Label>
                  <span className="text-muted-foreground text-xs">{temperature}</span>
                </div>
                <Slider
                  value={[temperature]}
                  onValueChange={([v]: number[]) => setTemperature(v)}
                  min={0}
                  max={2}
                  step={0.1}
                />
              </div>

              {/* Max Tokens */}
              <div className="flex flex-col gap-2">
                <Label>{t("maxTokens")}</Label>
                <Input
                  type="number"
                  value={maxTokens}
                  onChange={(e) => setMaxTokens(Number(e.target.value))}
                  min={1}
                  max={128000}
                />
              </div>

              {/* Top P */}
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("topP")}</Label>
                  <span className="text-muted-foreground text-xs">{topP}</span>
                </div>
                <Slider
                  value={[topP]}
                  onValueChange={([v]: number[]) => setTopP(v)}
                  min={0}
                  max={1}
                  step={0.05}
                />
              </div>
            </CollapsibleContent>
          </Collapsible>

          {/* Actions */}
          <div className="flex gap-2">
            <Button
              onClick={isLoading ? handleStop : handleSend}
              disabled={!model || !userMessage.trim()}
              className="flex-1"
              variant={isLoading ? "destructive" : "default"}
            >
              {isLoading ? (
                <>
                  <Square className="mr-2 h-4 w-4" />
                  {t("stop")}
                </>
              ) : (
                <>
                  <Send className="mr-2 h-4 w-4" />
                  {t("send")}
                </>
              )}
            </Button>
            <Button variant="outline" size="icon" onClick={handleClear} title={t("clear")}>
              <RotateCcw className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              onClick={handleCopyAsCurl}
              disabled={!model}
              title={t("copyAsCurl")}
            >
              <Terminal className="h-4 w-4" />
            </Button>
          </div>
        </div>

        {/* Right: Response */}
        <div className="flex flex-col gap-4 overflow-auto">
          <Card className="flex min-h-0 flex-1 flex-col">
            <CardHeader className="shrink-0 pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <Bot className="h-4 w-4" />
                {t("response")}
                {isLoading && (
                  <Badge variant="secondary" className="animate-pulse gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    {t("streaming")}
                  </Badge>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent className="min-h-0 flex-1">
              {error ? (
                <div className="text-destructive rounded-lg border border-red-200 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950">
                  <p className="font-medium">{t("error")}</p>
                  <p className="mt-1 opacity-80">{error}</p>
                </div>
              ) : response ? (
                <div className="relative">
                  <div className="prose prose-sm dark:prose-invert max-w-none break-words">
                    <Suspense
                      fallback={
                        <pre className="text-sm leading-relaxed break-words whitespace-pre-wrap">
                          {response}
                        </pre>
                      }
                    >
                      <LazyMarkdown>{response}</LazyMarkdown>
                    </Suspense>
                    {isLoading && (
                      <span className="ml-1 inline-block h-4 w-1.5 animate-pulse rounded-sm bg-current align-text-bottom" />
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="absolute top-0 right-0 h-7 w-7"
                    onClick={() => {
                      navigator.clipboard.writeText(response)
                      toast.success(t("actions.copied", { ns: "common" }))
                    }}
                  >
                    <Copy className="h-3.5 w-3.5" />
                  </Button>
                </div>
              ) : (
                <div className="text-muted-foreground flex flex-col items-center justify-center gap-2 py-16 text-sm">
                  <Bot className="h-10 w-10 opacity-30" />
                  <p>{t("noResponse")}</p>
                </div>
              )}
            </CardContent>
          </Card>

          {/* Stats */}
          {stats && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t("stats")}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                  <div>
                    <p className="text-muted-foreground text-xs">{t("latency")}</p>
                    <p className="font-mono font-medium">{(stats.latencyMs / 1000).toFixed(2)}s</p>
                  </div>
                  {stats.firstTokenMs !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("firstTokenTime")}</p>
                      <p className="font-mono font-medium">
                        {(stats.firstTokenMs / 1000).toFixed(2)}s
                      </p>
                    </div>
                  )}
                  {stats.inputTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("inputTokens")}</p>
                      <p className="font-mono font-medium">{stats.inputTokens.toLocaleString()}</p>
                    </div>
                  )}
                  {stats.outputTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("outputTokens")}</p>
                      <p className="font-mono font-medium">{stats.outputTokens.toLocaleString()}</p>
                    </div>
                  )}
                  {stats.totalTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("totalTokens")}</p>
                      <p className="font-mono font-medium">{stats.totalTokens.toLocaleString()}</p>
                    </div>
                  )}
                </div>
              </CardContent>
            </Card>
          )}
        </div>
      </div>
    </div>
  )
}

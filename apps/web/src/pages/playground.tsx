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
import { Suspense, useCallback, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { McpPanel } from "@/components/playground/mcp-panel"
import { ToolCallTimeline } from "@/components/playground/tool-call-timeline"
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
import { usePlaygroundChat } from "@/hooks/use-playground-chat"
import { getApiBaseUrl } from "@/lib/api-client"
import { LazyMarkdown } from "@/pages/logs/detail/code-block"

export default function PlaygroundPage() {
  const { t } = useTranslation("playground")
  const chat = usePlaygroundChat()
  const [showParams, setShowParams] = useState(false)

  const handleCopyAsCurl = useCallback(() => {
    const authKey = chat.customApiKey || chat.defaultApiKey || "YOUR_API_KEY"
    const baseUrl = getApiBaseUrl() || window.location.origin
    const messages = [
      ...(chat.systemPrompt ? [{ role: "system", content: chat.systemPrompt }] : []),
      { role: "user", content: chat.userMessage },
    ]
    const body: Record<string, unknown> = {
      model: chat.model,
      messages,
      stream: chat.resolvedStream,
      temperature: chat.temperature,
      max_tokens: chat.maxTokens,
      top_p: chat.topP,
    }
    if (chat.mcp.enabled && chat.mcp.mcpTools.length > 0) {
      body.tools = chat.mcp.mcpTools
    }
    const curl = `curl ${baseUrl}/v1/chat/completions \\
  -H "Content-Type: application/json" \\
  -H "Authorization: Bearer ${authKey}" \\
  -d '${JSON.stringify(body, null, 2)}'`
    navigator.clipboard.writeText(curl)
    toast.success(t("copiedCurl"))
  }, [chat, t])

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="grid min-h-0 flex-1 gap-6 lg:grid-cols-2">
        <div className="flex flex-col gap-4 overflow-auto p-1">
          <div className="flex flex-col gap-2">
            <Label>{t("model")}</Label>
            <Select value={chat.model} onValueChange={chat.setModel}>
              <SelectTrigger>
                <SelectValue placeholder={t("selectModel")} />
              </SelectTrigger>
              <SelectContent>
                {chat.models.map((m) => (
                  <SelectItem key={m} value={m}>
                    {m}
                  </SelectItem>
                ))}
                {chat.models.length === 0 && (
                  <div className="text-muted-foreground px-2 py-4 text-center text-sm">
                    {t("noModels")}
                  </div>
                )}
              </SelectContent>
            </Select>
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("systemPrompt")}</Label>
            <Textarea
              value={chat.systemPrompt}
              onChange={(e) => chat.setSystemPrompt(e.target.value)}
              placeholder={t("systemPromptPlaceholder")}
              rows={3}
              className="resize-none"
            />
          </div>

          <div className="flex flex-col gap-2">
            <Label>{t("userMessage")}</Label>
            <Textarea
              value={chat.userMessage}
              onChange={(e) => chat.setUserMessage(e.target.value)}
              placeholder={t("userMessagePlaceholder")}
              rows={6}
              className="min-h-[120px] resize-none"
              onKeyDown={(e) => {
                if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
                  e.preventDefault()
                  chat.send()
                }
              }}
            />
          </div>

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
              <div className="flex flex-col gap-2">
                <Label>{t("apiKey")}</Label>
                <Input
                  type="password"
                  value={chat.customApiKey}
                  onChange={(e) => chat.setCustomApiKey(e.target.value)}
                  placeholder={t("apiKeyPlaceholder")}
                />
              </div>

              <div className="flex items-center gap-2">
                <Switch
                  id="stream-toggle"
                  checked={chat.stream}
                  onCheckedChange={chat.setStream}
                  disabled={chat.mcp.enabled}
                />
                <Label htmlFor="stream-toggle">{t("stream")}</Label>
                {chat.mcp.enabled && (
                  <Badge variant="outline" className="text-[10px]">
                    MCP
                  </Badge>
                )}
              </div>

              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("temperature")}</Label>
                  <span className="text-muted-foreground text-xs">{chat.temperature}</span>
                </div>
                <Slider
                  value={[chat.temperature]}
                  onValueChange={([v]: number[]) => chat.setTemperature(v)}
                  min={0}
                  max={2}
                  step={0.1}
                />
              </div>

              <div className="flex flex-col gap-2">
                <Label>{t("maxTokens")}</Label>
                <Input
                  type="number"
                  value={chat.maxTokens}
                  onChange={(e) => chat.setMaxTokens(Number(e.target.value))}
                  min={1}
                  max={128000}
                />
              </div>

              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between">
                  <Label>{t("topP")}</Label>
                  <span className="text-muted-foreground text-xs">{chat.topP}</span>
                </div>
                <Slider
                  value={[chat.topP]}
                  onValueChange={([v]: number[]) => chat.setTopP(v)}
                  min={0}
                  max={1}
                  step={0.05}
                />
              </div>
            </CollapsibleContent>
          </Collapsible>

          <McpPanel mcp={chat.mcp} disabled={chat.isLoading} />

          <div className="flex gap-2">
            <Button
              onClick={chat.isLoading ? chat.stop : chat.send}
              disabled={!chat.model || !chat.userMessage.trim()}
              className="flex-1"
              variant={chat.isLoading ? "destructive" : "default"}
            >
              {chat.isLoading ? (
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
            <Button variant="outline" size="icon" onClick={chat.clear} title={t("clear")}>
              <RotateCcw className="h-4 w-4" />
            </Button>
            <Button
              variant="outline"
              size="icon"
              onClick={handleCopyAsCurl}
              disabled={!chat.model}
              title={t("copyAsCurl")}
            >
              <Terminal className="h-4 w-4" />
            </Button>
          </div>
        </div>

        <div className="flex flex-col gap-4 overflow-auto">
          <Card className="flex min-h-0 flex-1 flex-col">
            <CardHeader className="shrink-0 pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <Bot className="h-4 w-4" />
                {t("response")}
                {chat.isLoading && (
                  <Badge variant="secondary" className="animate-pulse gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    {chat.resolvedStream ? t("streaming") : t("sending")}
                  </Badge>
                )}
              </CardTitle>
            </CardHeader>
            <CardContent className="min-h-0 flex-1">
              {chat.error ? (
                <div className="text-destructive rounded-lg border border-red-200 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950">
                  <p className="font-medium">{t("error")}</p>
                  <p className="mt-1 opacity-80">{chat.error}</p>
                </div>
              ) : chat.response ? (
                <div className="relative">
                  <div className="prose prose-sm dark:prose-invert max-w-none break-words">
                    <Suspense
                      fallback={
                        <pre className="text-sm leading-relaxed break-words whitespace-pre-wrap">
                          {chat.response}
                        </pre>
                      }
                    >
                      <LazyMarkdown>{chat.response}</LazyMarkdown>
                    </Suspense>
                    {chat.isLoading && (
                      <span className="ml-1 inline-block h-4 w-1.5 animate-pulse rounded-sm bg-current align-text-bottom" />
                    )}
                  </div>
                  <Button
                    variant="ghost"
                    size="icon"
                    className="absolute top-0 right-0 h-7 w-7"
                    onClick={() => {
                      navigator.clipboard.writeText(chat.response)
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

          <ToolCallTimeline
            mode={chat.mcp.mode}
            items={chat.timeline}
            isLoading={chat.isLoading}
            canContinueManual={chat.canContinueManual}
            onExecuteTool={chat.executePendingTool}
            onContinue={chat.continueAfterTools}
          />

          {chat.stats && (
            <Card>
              <CardHeader className="pb-3">
                <CardTitle className="text-base">{t("stats")}</CardTitle>
              </CardHeader>
              <CardContent>
                <div className="grid grid-cols-2 gap-3 text-sm sm:grid-cols-3">
                  <div>
                    <p className="text-muted-foreground text-xs">{t("latency")}</p>
                    <p className="font-mono font-medium">
                      {(chat.stats.latencyMs / 1000).toFixed(2)}s
                    </p>
                  </div>
                  {chat.stats.firstTokenMs !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("firstTokenTime")}</p>
                      <p className="font-mono font-medium">
                        {(chat.stats.firstTokenMs / 1000).toFixed(2)}s
                      </p>
                    </div>
                  )}
                  {chat.stats.inputTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("inputTokens")}</p>
                      <p className="font-mono font-medium">
                        {chat.stats.inputTokens.toLocaleString()}
                      </p>
                    </div>
                  )}
                  {chat.stats.outputTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("outputTokens")}</p>
                      <p className="font-mono font-medium">
                        {chat.stats.outputTokens.toLocaleString()}
                      </p>
                    </div>
                  )}
                  {chat.stats.totalTokens !== undefined && (
                    <div>
                      <p className="text-muted-foreground text-xs">{t("totalTokens")}</p>
                      <p className="font-mono font-medium">
                        {chat.stats.totalTokens.toLocaleString()}
                      </p>
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

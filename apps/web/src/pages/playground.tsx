import {
  Bot,
  Copy,
  Loader2,
  Plug,
  RotateCcw,
  Send,
  Settings2,
  Square,
  Terminal,
} from "lucide-react"
import { Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { McpPanel } from "@/components/playground/mcp-panel"
import { ToolCallTimeline } from "@/components/playground/tool-call-timeline"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardFooter, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { Slider } from "@/components/ui/slider"
import { Switch } from "@/components/ui/switch"
import { Textarea } from "@/components/ui/textarea"
import { usePlaygroundChat } from "@/hooks/use-playground-chat"
import { getApiBaseUrl } from "@/lib/api-client"
import { LazyMarkdown } from "@/pages/logs/detail/code-block"

export default function PlaygroundPage() {
  const { t } = useTranslation("playground")
  const chat = usePlaygroundChat()
  const [settingsOpen, setSettingsOpen] = useState(false)
  const chatScrollRef = useRef<HTMLDivElement>(null)

  const visibleTurns = useMemo(() => {
    const turns = [...chat.conversation]
    if (chat.isLoading) {
      turns.push({ id: "assistant-pending", role: "assistant", content: chat.response })
    }
    return turns
  }, [chat.conversation, chat.isLoading, chat.response])

  const statsSummary = useMemo(() => {
    if (!chat.stats) return null
    const latencySec = (chat.stats.latencyMs / 1000).toFixed(2)
    const totalTokens = chat.stats.totalTokens ?? 0
    return `${latencySec}s · ${totalTokens.toLocaleString()} tok`
  }, [chat.stats])

  useEffect(() => {
    const el = chatScrollRef.current
    if (!el) return
    el.scrollTop = el.scrollHeight
  }, [visibleTurns.length, chat.response])

  const handleCopyAsCurl = useCallback(() => {
    const authKey = chat.customApiKey || chat.defaultApiKey || "YOUR_API_KEY"
    const baseUrl = getApiBaseUrl() || window.location.origin
    const body: Record<string, unknown> = {
      model: chat.model,
      messages: chat.requestMessagesForCurl,
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
      <Sheet open={settingsOpen} onOpenChange={setSettingsOpen}>
        <div className="mx-auto flex min-h-0 w-full max-w-6xl flex-1 flex-col gap-4">
          <Card className="flex min-h-0 flex-1 flex-col overflow-hidden rounded-2xl">
            <CardHeader className="shrink-0 border-b pb-3">
              <CardTitle className="flex items-center gap-2 text-base">
                <Bot className="h-4 w-4" />
                {t("response")}
                {chat.model && (
                  <Badge variant="outline" className="ml-2 max-w-[260px] truncate text-[10px]">
                    {chat.model}
                  </Badge>
                )}
                {chat.isLoading && (
                  <Badge variant="secondary" className="ml-2 animate-pulse gap-1">
                    <Loader2 className="h-3 w-3 animate-spin" />
                    {chat.resolvedStream ? t("streaming") : t("sending")}
                  </Badge>
                )}
                {!chat.isLoading && chat.sendState === "failed" && (
                  <Badge variant="destructive" className="ml-2">
                    {t("error")}
                  </Badge>
                )}
                <div className="ml-auto flex items-center gap-1">
                  <SheetTrigger asChild>
                    <Button variant="ghost" size="icon" className="h-7 w-7" title={t("parameters")}>
                      <Settings2 className="h-3.5 w-3.5" />
                    </Button>
                  </SheetTrigger>
                  {chat.response && (
                    <Button
                      variant="ghost"
                      size="icon"
                      className="h-7 w-7"
                      title={t("actions.copy", { ns: "common" })}
                      onClick={() => {
                        navigator.clipboard.writeText(chat.response)
                        toast.success(t("actions.copied", { ns: "common" }))
                      }}
                    >
                      <Copy className="h-3.5 w-3.5" />
                    </Button>
                  )}
                </div>
              </CardTitle>
            </CardHeader>
            <CardContent className="min-h-0 flex-1 overflow-hidden px-0 pb-0">
              {visibleTurns.length > 0 || chat.error ? (
                <div ref={chatScrollRef} className="h-full overflow-auto px-4 py-4">
                  <div className="mx-auto w-full max-w-3xl space-y-5">
                    {chat.error && (
                      <div className="text-destructive rounded-lg border border-red-200 bg-red-50 p-4 text-sm dark:border-red-900 dark:bg-red-950">
                        <div className="flex items-start justify-between gap-2">
                          <div>
                            <p className="font-medium">{t("error")}</p>
                            <p className="mt-1 opacity-80">{chat.error}</p>
                          </div>
                          {chat.canRetryLast && (
                            <Button
                              size="sm"
                              variant="outline"
                              onClick={chat.retryLast}
                              className="border-red-300 bg-transparent"
                            >
                              {t("actions.retry", { ns: "common", defaultValue: "重试" })}
                            </Button>
                          )}
                        </div>
                      </div>
                    )}

                    {visibleTurns.map((turn) => {
                      const isUser = turn.role === "user"
                      const isPendingAssistant = !isUser && turn.id === "assistant-pending"
                      const hasAssistantText = !!turn.content?.trim()

                      return isUser ? (
                        <div key={turn.id} className="flex justify-end">
                          <div className="bg-muted/40 max-w-[78%] rounded-2xl border px-3 py-2 text-sm leading-relaxed break-words whitespace-pre-wrap">
                            {turn.content}
                          </div>
                        </div>
                      ) : (
                        <div
                          key={turn.id}
                          className="prose prose-sm dark:prose-invert max-w-none break-words"
                        >
                          {hasAssistantText ? (
                            <Suspense
                              fallback={
                                <pre className="text-sm leading-relaxed break-words whitespace-pre-wrap">
                                  {turn.content}
                                </pre>
                              }
                            >
                              <LazyMarkdown>{turn.content}</LazyMarkdown>
                            </Suspense>
                          ) : (
                            isPendingAssistant && (
                              <div className="text-muted-foreground flex items-center gap-2 text-sm">
                                <Loader2 className="h-4 w-4 animate-spin" />
                                <span>{t("sending")}</span>
                              </div>
                            )
                          )}
                          {isPendingAssistant && hasAssistantText && (
                            <span className="ml-1 inline-block h-4 w-1.5 animate-pulse rounded-sm bg-current align-text-bottom" />
                          )}
                        </div>
                      )
                    })}
                  </div>
                </div>
              ) : (
                <div className="text-muted-foreground flex h-full flex-col items-center justify-center gap-2 px-4 py-16 text-sm">
                  <Bot className="h-10 w-10 opacity-30" />
                  <p>{t("noResponse")}</p>
                </div>
              )}
            </CardContent>
            <CardFooter className="bg-background block border-t px-4 pt-3 pb-4">
              <div className="bg-card mx-auto w-full max-w-3xl rounded-2xl border p-3 shadow-sm">
                <Textarea
                  value={chat.userMessage}
                  onChange={(e) => chat.setUserMessage(e.target.value)}
                  placeholder={t("userMessagePlaceholder")}
                  rows={3}
                  className="min-h-[84px] resize-none border-0 bg-transparent p-0 shadow-none focus-visible:ring-0"
                  onKeyDown={(e) => {
                    if ((e.nativeEvent as { isComposing?: boolean }).isComposing) return
                    if (e.key === "Enter" && !e.shiftKey) {
                      e.preventDefault()
                      if (chat.isLoading) {
                        chat.stop()
                        return
                      }
                      if (!chat.model || !chat.userMessage.trim() || chat.hasPendingToolCalls)
                        return
                      chat.send()
                    }
                  }}
                />
                <div className="mt-2 flex items-center justify-between gap-2 border-t pt-2">
                  <div className="text-muted-foreground flex items-center gap-1">
                    <SheetTrigger asChild>
                      <Button variant="ghost" size="icon" title={t("parameters")}>
                        <Settings2 className="h-4 w-4" />
                      </Button>
                    </SheetTrigger>
                    <Button variant="ghost" size="icon" onClick={chat.clear} title={t("clear")}>
                      <RotateCcw className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      className={chat.mcp.enabled ? "text-emerald-600" : undefined}
                      onClick={() => chat.mcp.setEnabled(!chat.mcp.enabled)}
                      title={t("mcp.enable")}
                    >
                      <Plug className="h-4 w-4" />
                    </Button>
                    <Button
                      variant="ghost"
                      size="icon"
                      onClick={handleCopyAsCurl}
                      disabled={!chat.model}
                      title={t("copyAsCurl")}
                    >
                      <Terminal className="h-4 w-4" />
                    </Button>
                    {chat.mcp.enabled && (
                      <Badge variant="outline" className="ml-1 text-[10px]">
                        MCP
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-2">
                    {statsSummary && (
                      <span className="text-muted-foreground text-xs">{statsSummary}</span>
                    )}
                    <Button
                      onClick={chat.isLoading ? chat.stop : chat.send}
                      disabled={!chat.model || !chat.userMessage.trim() || chat.hasPendingToolCalls}
                      size="icon"
                      className="h-8 w-8 rounded-full"
                      variant={chat.isLoading ? "destructive" : "default"}
                      title={chat.isLoading ? t("stop") : t("send")}
                    >
                      {chat.isLoading ? (
                        <Square className="h-4 w-4" />
                      ) : (
                        <Send className="h-4 w-4" />
                      )}
                    </Button>
                  </div>
                </div>
              </div>
            </CardFooter>
          </Card>

          {(chat.timeline.length > 0 || chat.hasPendingToolCalls) && (
            <ToolCallTimeline
              mode={chat.mcp.mode}
              items={chat.timeline}
              isLoading={chat.isLoading}
              canContinueManual={chat.canContinueManual}
              onExecuteTool={chat.executePendingTool}
              onContinue={chat.continueAfterTools}
            />
          )}

          <SheetContent className="w-[92vw] max-w-none p-0 sm:max-w-lg">
            <SheetHeader>
              <SheetTitle>{t("parameters")}</SheetTitle>
              <SheetDescription>{t("description")}</SheetDescription>
            </SheetHeader>
            <div className="flex-1 space-y-4 overflow-auto px-4 pb-4">
              <div className="space-y-2">
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

              <div className="space-y-2">
                <Label>{t("systemPrompt")}</Label>
                <Textarea
                  value={chat.systemPrompt}
                  onChange={(e) => chat.setSystemPrompt(e.target.value)}
                  placeholder={t("systemPromptPlaceholder")}
                  rows={3}
                  className="resize-none"
                />
              </div>

              <div className="space-y-2">
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

              <div className="space-y-2">
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

              <div className="space-y-2">
                <Label>{t("maxTokens")}</Label>
                <Input
                  type="number"
                  value={chat.maxTokens}
                  onChange={(e) => chat.setMaxTokens(Number(e.target.value))}
                  min={1}
                  max={128000}
                />
              </div>

              <div className="space-y-2">
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

              <McpPanel mcp={chat.mcp} disabled={chat.isLoading} />
            </div>
          </SheetContent>
        </div>
      </Sheet>
    </div>
  )
}

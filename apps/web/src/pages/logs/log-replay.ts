import { useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { replayLog } from "@/lib/api-client"

export function useLogReplay() {
  const { t } = useTranslation("logs")
  const [replayResult, setReplayResult] = useState<string | null>(null)
  const [replaying, setReplaying] = useState(false)

  const handleReplay = async (logId: number, onSuccess: () => void) => {
    setReplaying(true)
    try {
      const resp = await replayLog(logId)
      const contentType = resp.headers.get("content-type") ?? ""

      if (contentType.includes("text/event-stream")) {
        // Streaming response: read and concatenate text deltas
        const reader = resp.body?.getReader()
        if (!reader) throw new Error("No response body")
        const decoder = new TextDecoder()
        let text = ""
        while (true) {
          const { done, value } = await reader.read()
          if (done) break
          const chunk = decoder.decode(value, { stream: true })
          for (const line of chunk.split("\n")) {
            if (line.startsWith("data: ") && line !== "data: [DONE]") {
              try {
                const obj = JSON.parse(line.slice(6))
                const delta = obj.choices?.[0]?.delta?.content
                if (delta) text += delta
              } catch {
                /* skip */
              }
            }
          }
        }
        setReplayResult(text || t("detail.emptyResponse"))
      } else {
        const data = (await resp.json()) as {
          success: boolean
          data?: { response: unknown; truncated: boolean }
          error?: string
        }
        if (!data.success) throw new Error(data.error ?? t("detail.replayFailed"))
        setReplayResult(JSON.stringify(data.data?.response, null, 2))
      }
      onSuccess()
    } catch (err) {
      toast.error(err instanceof Error ? err.message : t("detail.replayFailed"))
    } finally {
      setReplaying(false)
    }
  }

  return { replayResult, replaying, handleReplay }
}

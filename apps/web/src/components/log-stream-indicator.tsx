import { Pause, Play } from "lucide-react"
import { useTranslation } from "react-i18next"

interface LogStreamIndicatorProps {
  isLive: boolean
  isPaused: boolean
  streamCount: number
  onTogglePause: () => void
}

export function LogStreamIndicator({
  isLive,
  isPaused,
  streamCount,
  onTogglePause,
}: LogStreamIndicatorProps) {
  const { t } = useTranslation()

  return (
    <div className="flex items-center gap-2">
      {isLive && (
        <div className="flex items-center gap-1.5">
          <div
            className={`h-2 w-2 rounded-full ${isPaused ? "bg-yellow-500" : "animate-pulse bg-green-500"}`}
          />
          <span className="text-muted-foreground text-xs font-medium">
            {isPaused ? t("logs.stream.paused") : t("logs.stream.live")}
          </span>
          {streamCount > 0 && (
            <span className="text-muted-foreground text-xs">
              ({streamCount} {t("logs.stream.streaming")})
            </span>
          )}
        </div>
      )}
      {isLive && (
        <button
          type="button"
          onClick={onTogglePause}
          className="hover:bg-muted rounded-md p-1 transition-colors"
          title={isPaused ? t("logs.stream.resume") : t("logs.stream.pause")}
        >
          {isPaused ? <Play className="h-3.5 w-3.5" /> : <Pause className="h-3.5 w-3.5" />}
        </button>
      )}
    </div>
  )
}

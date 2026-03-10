import type { DateRange } from "react-day-picker"
import { Calendar as CalendarIcon, Clock, Dot, X } from "lucide-react"
import { useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import { Popover, PopoverContent, PopoverTrigger } from "@/components/ui/popover"
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip"
import { cn } from "@/lib/utils"

export const TIME_RANGE_PRESETS = [
  { label: "1h", seconds: 3600 },
  { label: "6h", seconds: 21600 },
  { label: "24h", seconds: 86400 },
  { label: "7d", seconds: 604800 },
  { label: "30d", seconds: 2592000 },
] as const

type PresetType = "today" | "yesterday" | (typeof TIME_RANGE_PRESETS)[number]["label"]

/** Format a unix timestamp to compact "MM/DD HH:mm" */
export function formatCompactDate(ts: number): string {
  const d = new Date(ts * 1000)
  const month = String(d.getMonth() + 1).padStart(2, "0")
  const day = String(d.getDate()).padStart(2, "0")
  const hours = String(d.getHours()).padStart(2, "0")
  const minutes = String(d.getMinutes()).padStart(2, "0")
  return `${month}/${day} ${hours}:${minutes}`
}

/** Detect if the current range matches a known preset */
export function detectPreset(
  from: number | undefined,
  to: number | undefined,
  now?: number,
): (typeof TIME_RANGE_PRESETS)[number] | null {
  if (from === undefined || to === undefined) return null
  const duration = to - from
  const currentTime = now ?? Math.floor(Date.now() / 1000)
  if (Math.abs(to - currentTime) > 60) return null
  return TIME_RANGE_PRESETS.find((p) => p.seconds === duration) ?? null
}

/** Detect if the current range matches "today" or "yesterday" */
function detectDayPreset(from: number | undefined, to: number | undefined): PresetType | null {
  if (from === undefined || to === undefined) return null

  const now = new Date()

  // Check "today": from = start of today, to = end of today (or near-now)
  const todayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate())
  const todayEnd = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59)
  const todayStartTs = Math.floor(todayStart.getTime() / 1000)
  const todayEndTs = Math.floor(todayEnd.getTime() / 1000)

  if (
    Math.abs(from - todayStartTs) <= 60 &&
    (Math.abs(to - todayEndTs) <= 60 || Math.abs(to - Math.floor(Date.now() / 1000)) <= 60)
  ) {
    return "today"
  }

  // Check "yesterday"
  const yesterdayStart = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
  const yesterdayEnd = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59)
  const yStartTs = Math.floor(yesterdayStart.getTime() / 1000)
  const yEndTs = Math.floor(yesterdayEnd.getTime() / 1000)

  if (Math.abs(from - yStartTs) <= 60 && Math.abs(to - yEndTs) <= 60) {
    return "yesterday"
  }

  // Check duration presets
  const preset = detectPreset(from, to)
  if (preset) return preset.label as PresetType

  return null
}

/** Format the range summary for display (i18n-aware) */
export function formatRangeSummary(
  from: number | undefined,
  to: number | undefined,
  t: (key: string, options?: Record<string, unknown>) => string,
  now?: number,
): string {
  if (from === undefined && to === undefined) return t("timeRange.label")

  const preset = detectPreset(from, to, now)
  if (preset) return t("timeRange.last", { label: preset.label })

  if (from !== undefined && to !== undefined) {
    return `${formatCompactDate(from)} – ${formatCompactDate(to)}`
  }
  if (from !== undefined) return `After ${formatCompactDate(from)}`
  return `Before ${formatCompactDate(to!)}`
}

/** Convert unix seconds to { date, hours, minutes } in local timezone */
function unixToLocal(ts: number) {
  const d = new Date(ts * 1000)
  return {
    date: new Date(d.getFullYear(), d.getMonth(), d.getDate()),
    hours: String(d.getHours()).padStart(2, "0"),
    minutes: String(d.getMinutes()).padStart(2, "0"),
  }
}

/** Combine a Date and time strings into unix seconds */
function localToUnix(date: Date, hours: string, minutes: string): number {
  const d = new Date(
    date.getFullYear(),
    date.getMonth(),
    date.getDate(),
    Number.parseInt(hours) || 0,
    Number.parseInt(minutes) || 0,
  )
  return Math.floor(d.getTime() / 1000)
}

interface TimeRangePickerProps {
  from?: number
  to?: number
  onChange: (from?: number, to?: number) => void
}

export function TimeRangePicker({ from, to, onChange }: TimeRangePickerProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const [dateRange, setDateRange] = useState<DateRange | undefined>()
  const [month, setMonth] = useState(() => (from ? new Date(from * 1000) : new Date()))
  const [fromTime, setFromTime] = useState({ hours: "00", minutes: "00" })
  const [toTime, setToTime] = useState({ hours: "23", minutes: "59" })

  const hasRange = from !== undefined || to !== undefined
  const summary = formatRangeSummary(from, to, t)
  const activePreset = useMemo(() => detectDayPreset(from, to), [from, to])

  const handleOpenChange = (isOpen: boolean) => {
    if (isOpen) {
      if (from !== undefined) {
        const f = unixToLocal(from)
        setFromTime({ hours: f.hours, minutes: f.minutes })
        setMonth(f.date)
        if (to !== undefined) {
          const t = unixToLocal(to)
          setToTime({ hours: t.hours, minutes: t.minutes })
          setDateRange({ from: f.date, to: t.date })
        } else {
          setToTime({ hours: "23", minutes: "59" })
          setDateRange({ from: f.date, to: undefined })
        }
      } else if (to !== undefined) {
        const t = unixToLocal(to)
        setToTime({ hours: t.hours, minutes: t.minutes })
        setFromTime({ hours: "00", minutes: "00" })
        setDateRange({ from: undefined, to: t.date })
      } else {
        setDateRange(undefined)
        setFromTime({ hours: "00", minutes: "00" })
        setToTime({ hours: "23", minutes: "59" })
        setMonth(new Date())
      }
    }
    setOpen(isOpen)
  }

  const handlePreset = (seconds: number) => {
    const now = Math.floor(Date.now() / 1000)
    onChange(now - seconds, now)
    setOpen(false)
  }

  const handleDayPreset = (preset: "today" | "yesterday") => {
    const now = new Date()
    if (preset === "today") {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59)
      onChange(Math.floor(start.getTime() / 1000), Math.floor(end.getTime() / 1000))
    } else {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59)
      onChange(Math.floor(start.getTime() / 1000), Math.floor(end.getTime() / 1000))
    }
    setOpen(false)
  }

  /** Compute the actual date range string for a preset tooltip */
  const getPresetRange = (seconds: number): string => {
    const now = Math.floor(Date.now() / 1000)
    return `${formatCompactDate(now - seconds)} – ${formatCompactDate(now)}`
  }

  const getDayPresetRange = (preset: "today" | "yesterday"): string => {
    const now = new Date()
    if (preset === "today") {
      const start = new Date(now.getFullYear(), now.getMonth(), now.getDate())
      const end = new Date(now.getFullYear(), now.getMonth(), now.getDate(), 23, 59, 59)
      return `${formatCompactDate(Math.floor(start.getTime() / 1000))} – ${formatCompactDate(Math.floor(end.getTime() / 1000))}`
    }
    const start = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1)
    const end = new Date(now.getFullYear(), now.getMonth(), now.getDate() - 1, 23, 59, 59)
    return `${formatCompactDate(Math.floor(start.getTime() / 1000))} – ${formatCompactDate(Math.floor(end.getTime() / 1000))}`
  }

  const handleApply = () => {
    if (!dateRange?.from) {
      onChange(undefined, undefined)
      setOpen(false)
      return
    }
    const newFrom = localToUnix(dateRange.from, fromTime.hours, fromTime.minutes)
    const newTo = dateRange.to ? localToUnix(dateRange.to, toTime.hours, toTime.minutes) : undefined
    onChange(newFrom, newTo)
    setOpen(false)
  }

  const handleClear = (e: React.MouseEvent | React.KeyboardEvent) => {
    e.stopPropagation()
    onChange(undefined, undefined)
  }

  return (
    <Popover open={open} onOpenChange={handleOpenChange}>
      <PopoverTrigger asChild>
        <Button
          variant="outline"
          size="sm"
          className={cn("h-8 gap-1.5 text-xs", hasRange && "border-primary")}
        >
          <CalendarIcon className="h-3.5 w-3.5" />
          <span className={hasRange ? "font-bold" : "text-muted-foreground"}>{summary}</span>
          {hasRange && (
            <span
              role="button"
              tabIndex={0}
              className="hover:bg-muted -mr-1 ml-0.5 rounded-sm p-0.5"
              onClick={handleClear}
              onKeyDown={(e) => {
                if (e.key === "Enter" || e.key === " ") handleClear(e)
              }}
            >
              <X className="h-3 w-3" />
            </span>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent align="start" className="flex w-auto gap-0 p-0">
        {/* Presets */}
        <TooltipProvider delayDuration={400}>
          <div className="border-border flex flex-col gap-0.5 border-r p-3">
            <span className="text-muted-foreground mb-1 px-2 text-[10px] font-bold tracking-wider uppercase">
              {t("timeRange.presets")}
            </span>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="xs"
                  className={cn(
                    "justify-start gap-1.5 px-2",
                    activePreset === "today" && "bg-accent text-accent-foreground font-medium",
                  )}
                  onClick={() => handleDayPreset("today")}
                >
                  <Clock className="h-3 w-3" />
                  {t("timeRange.today")}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs">
                {getDayPresetRange("today")}
              </TooltipContent>
            </Tooltip>
            <Tooltip>
              <TooltipTrigger asChild>
                <Button
                  variant="ghost"
                  size="xs"
                  className={cn(
                    "justify-start gap-1.5 px-2",
                    activePreset === "yesterday" && "bg-accent text-accent-foreground font-medium",
                  )}
                  onClick={() => handleDayPreset("yesterday")}
                >
                  <Clock className="h-3 w-3" />
                  {t("timeRange.yesterday")}
                </Button>
              </TooltipTrigger>
              <TooltipContent side="right" className="text-xs">
                {getDayPresetRange("yesterday")}
              </TooltipContent>
            </Tooltip>
            <div className="border-border my-1 border-t" />
            {TIME_RANGE_PRESETS.map((preset) => (
              <Tooltip key={preset.label}>
                <TooltipTrigger asChild>
                  <Button
                    variant="ghost"
                    size="xs"
                    className={cn(
                      "justify-start gap-1.5 px-2",
                      activePreset === preset.label &&
                        "bg-accent text-accent-foreground font-medium",
                    )}
                    onClick={() => handlePreset(preset.seconds)}
                  >
                    <CalendarIcon className="h-3 w-3" />
                    {t("timeRange.last", { label: preset.label })}
                  </Button>
                </TooltipTrigger>
                <TooltipContent side="right" className="text-xs">
                  {getPresetRange(preset.seconds)}
                </TooltipContent>
              </Tooltip>
            ))}
            {hasRange && !activePreset && (
              <>
                <div className="border-border my-1 border-t" />
                <div className="flex items-center gap-1.5 px-2 py-1">
                  <div className="bg-primary h-1.5 w-1.5 rounded-full" />
                  <span className="text-muted-foreground text-xs font-medium">
                    {t("timeRange.custom")}
                  </span>
                </div>
              </>
            )}
          </div>
        </TooltipProvider>

        {/* Calendar + Time */}
        <div className="flex flex-col">
          <Calendar
            mode="range"
            selected={dateRange}
            onSelect={setDateRange}
            month={month}
            onMonthChange={setMonth}
            numberOfMonths={1}
            footer={
              <button
                type="button"
                className="text-muted-foreground hover:text-foreground mt-1 flex w-full items-center justify-center gap-0.5 text-xs font-medium transition-colors"
                onClick={() => setMonth(new Date())}
              >
                <Dot className="h-4 w-4" />
                {t("timeRange.today")}
              </button>
            }
          />

          {/* Time inputs */}
          <div className="border-border flex items-center gap-4 border-t px-4 py-3">
            <div className="flex items-center gap-1.5">
              <span className="text-muted-foreground text-xs font-medium">
                {t("timeRange.from")}
              </span>
              <input
                type="text"
                inputMode="numeric"
                maxLength={2}
                value={fromTime.hours}
                onChange={(e) => {
                  const v = e.target.value.replace(/\D/g, "").slice(0, 2)
                  setFromTime((p) => ({ ...p, hours: v }))
                }}
                onBlur={(e) => {
                  const n = Math.min(23, Math.max(0, Number.parseInt(e.target.value) || 0))
                  setFromTime((p) => ({ ...p, hours: String(n).padStart(2, "0") }))
                }}
                className="border-input bg-background w-7 rounded border px-1 py-0.5 text-center font-mono text-xs"
              />
              <span className="text-muted-foreground text-xs">:</span>
              <input
                type="text"
                inputMode="numeric"
                maxLength={2}
                value={fromTime.minutes}
                onChange={(e) => {
                  const v = e.target.value.replace(/\D/g, "").slice(0, 2)
                  setFromTime((p) => ({ ...p, minutes: v }))
                }}
                onBlur={(e) => {
                  const n = Math.min(59, Math.max(0, Number.parseInt(e.target.value) || 0))
                  setFromTime((p) => ({ ...p, minutes: String(n).padStart(2, "0") }))
                }}
                className="border-input bg-background w-7 rounded border px-1 py-0.5 text-center font-mono text-xs"
              />
            </div>

            <div className="flex items-center gap-1.5">
              <span className="text-muted-foreground text-xs font-medium">{t("timeRange.to")}</span>
              <input
                type="text"
                inputMode="numeric"
                maxLength={2}
                value={toTime.hours}
                onChange={(e) => {
                  const v = e.target.value.replace(/\D/g, "").slice(0, 2)
                  setToTime((p) => ({ ...p, hours: v }))
                }}
                onBlur={(e) => {
                  const n = Math.min(23, Math.max(0, Number.parseInt(e.target.value) || 0))
                  setToTime((p) => ({ ...p, hours: String(n).padStart(2, "0") }))
                }}
                className="border-input bg-background w-7 rounded border px-1 py-0.5 text-center font-mono text-xs"
              />
              <span className="text-muted-foreground text-xs">:</span>
              <input
                type="text"
                inputMode="numeric"
                maxLength={2}
                value={toTime.minutes}
                onChange={(e) => {
                  const v = e.target.value.replace(/\D/g, "").slice(0, 2)
                  setToTime((p) => ({ ...p, minutes: v }))
                }}
                onBlur={(e) => {
                  const n = Math.min(59, Math.max(0, Number.parseInt(e.target.value) || 0))
                  setToTime((p) => ({ ...p, minutes: String(n).padStart(2, "0") }))
                }}
                className="border-input bg-background w-7 rounded border px-1 py-0.5 text-center font-mono text-xs"
              />
            </div>

            <Button size="xs" className="ml-auto" onClick={handleApply}>
              {t("timeRange.apply")}
            </Button>
          </div>
        </div>
      </PopoverContent>
    </Popover>
  )
}

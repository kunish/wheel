import { Search, X } from "lucide-react"
import { useTranslation } from "react-i18next"
import { ModelCombobox } from "@/components/model-combobox"
import { formatRangeSummary, TimeRangePicker } from "@/components/time-range-picker"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { useLogQueryContext } from "./log-query-context"

export function LogFilterBar() {
  const { t } = useTranslation("logs")
  const {
    keyword,
    keywordInput,
    setKeywordInput,
    model,
    status,
    channelId,
    startTime,
    endTime,
    hasFilters,
    channels,
    modelOptions,
    updateFilter,
    debouncedUpdateFilter,
    navigate,
    pathname,
  } = useLogQueryContext()

  const handleClearAll = () => {
    navigate(pathname, { replace: true })
    setKeywordInput("")
  }

  return (
    <div className="flex flex-col gap-2">
      {/* Filters: search, model, channel, status, time */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[200px] flex-1">
          <Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
          <Input
            placeholder={t("searchPlaceholder")}
            aria-label={t("searchPlaceholder")}
            value={keywordInput}
            onChange={(e) => {
              setKeywordInput(e.target.value)
              debouncedUpdateFilter("q", e.target.value)
            }}
            className="pl-9"
          />
          {keywordInput && (
            <Button
              variant="ghost"
              size="icon"
              aria-label="Clear search"
              className="absolute top-1/2 right-1 h-7 w-7 -translate-y-1/2"
              onClick={() => {
                setKeywordInput("")
                updateFilter({ q: undefined })
              }}
            >
              <X className="h-3 w-3" />
            </Button>
          )}
        </div>
        <ModelCombobox
          models={modelOptions}
          value={model}
          onChange={(v) => updateFilter({ model: v || undefined })}
        />
        <Select
          value={channelId ? String(channelId) : "all"}
          onValueChange={(v) => {
            updateFilter({ channel: v === "all" ? undefined : v })
          }}
        >
          <SelectTrigger className="w-36">
            <SelectValue placeholder={t("filter.allChannels")} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("filter.allChannels")}</SelectItem>
            {channels.map((ch) => (
              <SelectItem key={ch.id} value={String(ch.id)}>
                {ch.name}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        <Select
          value={status}
          onValueChange={(v) => {
            updateFilter({ status: v })
          }}
        >
          <SelectTrigger className="w-28">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="all">{t("filter.all")}</SelectItem>
            <SelectItem value="success">{t("filter.success")}</SelectItem>
            <SelectItem value="error">{t("filter.error")}</SelectItem>
          </SelectContent>
        </Select>
        <TimeRangePicker
          from={startTime}
          to={endTime}
          onChange={(from, to) => updateFilter({ from: from ?? undefined, to: to ?? undefined })}
        />
      </div>

      {/* Active filter chips */}
      {hasFilters && (
        <div className="flex flex-wrap gap-1.5">
          {keyword && (
            <Badge variant="secondary" className="gap-1 pr-1">
              {t("chips.search", { value: keyword })}
              <button
                onClick={() => {
                  setKeywordInput("")
                  updateFilter({ q: undefined })
                }}
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          {model && (
            <Badge variant="secondary" className="gap-1 pr-1">
              {t("chips.model", { value: model })}
              <button
                onClick={() => {
                  updateFilter({ model: undefined })
                }}
              >
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          {channelId && (
            <Badge variant="secondary" className="gap-1 pr-1">
              {t("chips.channel", {
                value: channels.find((c) => c.id === channelId)?.name ?? channelId,
              })}
              <button onClick={() => updateFilter({ channel: undefined })}>
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          {status !== "all" && (
            <Badge variant="secondary" className="gap-1 pr-1">
              {t("chips.status", { value: status })}
              <button onClick={() => updateFilter({ status: "all" })}>
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          {(startTime || endTime) && (
            <Badge variant="secondary" className="gap-1 pr-1">
              {t("chips.time", { value: formatRangeSummary(startTime, endTime, t) })}
              <button onClick={() => updateFilter({ from: undefined, to: undefined })}>
                <X className="h-3 w-3" />
              </button>
            </Badge>
          )}
          <Button variant="ghost" size="sm" className="h-6 px-2 text-xs" onClick={handleClearAll}>
            {t("chips.clearAll")}
          </Button>
        </div>
      )}
    </div>
  )
}

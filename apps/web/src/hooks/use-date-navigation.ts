import type { StatsDaily, StatsHourly } from "@/lib/api"
import type { DayData, HeatmapView } from "@/pages/dashboard/types"
import { useQuery } from "@tanstack/react-query"
import { useCallback, useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { getHourlyStats } from "@/lib/api"
import {
  buildDayData,
  getFirstDayOfWeek,
  getStoredView,
  HEATMAP_VIEW_KEY,
  toDateStr,
} from "@/pages/dashboard/types"

export function useDateNavigation(dataMap: Map<string, StatsDaily>) {
  const { t, i18n } = useTranslation("dashboard")
  const { t: tc } = useTranslation("common")
  const navigate = useNavigate()
  const firstDay = useMemo(() => getFirstDayOfWeek(i18n.language), [i18n.language])

  const [view, setView] = useState<HeatmapView>(getStoredView)
  useEffect(() => {
    try {
      localStorage.setItem(HEATMAP_VIEW_KEY, view)
    } catch {}
  }, [view])

  const [selectedDateStr, setSelectedDateStr] = useState<string | null>(null)
  const [weekOffset, setWeekOffset] = useState(0)
  const [monthOffset, setMonthOffset] = useState(0)
  const [yearOffset, setYearOffset] = useState(0)

  const [today] = useState(() => new Date())

  const weekdayLabelsRaw = useMemo(
    () => [
      tc("weekdays.sun"),
      tc("weekdays.mon"),
      tc("weekdays.tue"),
      tc("weekdays.wed"),
      tc("weekdays.thu"),
      tc("weekdays.fri"),
      tc("weekdays.sat"),
    ],
    [tc],
  )

  const weekdayLabels = useMemo(
    () => [...weekdayLabelsRaw.slice(firstDay), ...weekdayLabelsRaw.slice(0, firstDay)],
    [weekdayLabelsRaw, firstDay],
  )

  const weekdaysFull = useMemo(
    () => [
      t("weekdaysFull.sunday"),
      t("weekdaysFull.monday"),
      t("weekdaysFull.tuesday"),
      t("weekdaysFull.wednesday"),
      t("weekdaysFull.thursday"),
      t("weekdaysFull.friday"),
      t("weekdaysFull.saturday"),
    ],
    [t],
  )

  const viewLabels = useMemo(
    () =>
      ({
        day: t("activity.day"),
        week: t("activity.week"),
        month: t("activity.month"),
        year: t("activity.year"),
      }) as Record<HeatmapView, string>,
    [t],
  )

  // ── Year view ──
  const yearAnchor = useMemo(() => {
    const d = new Date(today.getFullYear() + yearOffset, 11, 31)
    if (yearOffset === 0) return today
    return d
  }, [today, yearOffset])

  const yearLabel = useMemo(() => `${today.getFullYear() + yearOffset}`, [today, yearOffset])

  const yearDays = useMemo(() => {
    const anchor = yearAnchor
    const anchorDay = (anchor.getDay() - firstDay + 7) % 7
    const start = new Date(anchor)
    start.setDate(start.getDate() - anchorDay - 52 * 7)
    const result: DayData[] = []
    for (let i = 0; i < 53 * 7; i++) {
      const d = new Date(start)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today, yearAnchor, firstDay])

  // ── Month view ──
  const monthAnchor = useMemo(
    () => new Date(today.getFullYear(), today.getMonth() + monthOffset, 1),
    [today, monthOffset],
  )

  const monthLabel = useMemo(
    () =>
      `${monthAnchor.getFullYear()}-${(monthAnchor.getMonth() + 1).toString().padStart(2, "0")}`,
    [monthAnchor],
  )

  const monthDays = useMemo(() => {
    const year = monthAnchor.getFullYear()
    const month = monthAnchor.getMonth()
    const monthFirstDate = new Date(year, month, 1)
    const lastDay = new Date(year, month + 1, 0)
    const startPad = (monthFirstDate.getDay() - firstDay + 7) % 7
    const result: (DayData | null)[] = []
    for (let i = 0; i < startPad; i++) result.push(null)
    for (let d = 1; d <= lastDay.getDate(); d++) {
      const date = new Date(year, month, d)
      result.push(buildDayData(date, dataMap, today))
    }
    return result
  }, [dataMap, today, monthAnchor, firstDay])

  // ── Week view ──
  const weekStart = useMemo(() => {
    const todayDay = today.getDay()
    const diff = (todayDay - firstDay + 7) % 7
    const start = new Date(today)
    start.setDate(start.getDate() - diff + weekOffset * 7)
    return start
  }, [today, weekOffset, firstDay])

  const weekLabel = useMemo(() => {
    const end = new Date(weekStart)
    end.setDate(end.getDate() + 6)
    const fmt = (d: Date) =>
      `${d.getFullYear()}-${(d.getMonth() + 1).toString().padStart(2, "0")}-${d.getDate().toString().padStart(2, "0")}`
    return `${fmt(weekStart)} ~ ${fmt(end)}`
  }, [weekStart])

  const weekDays = useMemo(() => {
    const result: DayData[] = []
    for (let i = 0; i < 7; i++) {
      const d = new Date(weekStart)
      d.setDate(d.getDate() + i)
      result.push(buildDayData(d, dataMap, today))
    }
    return result
  }, [dataMap, today, weekStart])

  // ── Day view ──
  const selectedDayDateStr = selectedDateStr ?? toDateStr(today)

  const { data: dayHourlyData } = useQuery({
    queryKey: ["stats", "hourly", selectedDayDateStr, selectedDayDateStr],
    queryFn: () => getHourlyStats(selectedDayDateStr, selectedDayDateStr),
    enabled: view === "day",
  })

  const dayHourlyMap = useMemo(() => {
    const raw = dayHourlyData?.data
    if (!raw) return new Map<number, StatsHourly>()
    const map = new Map<number, StatsHourly>()
    for (const s of raw) {
      if (s.date === selectedDayDateStr) map.set(s.hour, s)
    }
    return map
  }, [dayHourlyData, selectedDayDateStr])

  const selectedDayData = useMemo(
    () => dataMap.get(selectedDayDateStr) ?? null,
    [dataMap, selectedDayDateStr],
  )

  const selectedDisplayDate = useMemo(() => {
    const ds = selectedDayDateStr
    return `${ds.slice(0, 4)}-${ds.slice(4, 6)}-${ds.slice(6, 8)}`
  }, [selectedDayDateStr])

  const selectedDayWeekday = useMemo(() => {
    const ds = selectedDayDateStr
    const d = new Date(
      Number.parseInt(ds.slice(0, 4)),
      Number.parseInt(ds.slice(4, 6)) - 1,
      Number.parseInt(ds.slice(6, 8)),
    )
    return weekdaysFull[d.getDay()]
  }, [selectedDayDateStr, weekdaysFull])

  // ── Navigation callbacks ──
  const drillIntoDay = useCallback((dateStr: string) => {
    setSelectedDateStr(dateStr)
    setView("day")
  }, [])

  const navigateToDay = useCallback(
    (dateStr: string) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d).getTime() / 1000)
      navigate(`/logs?from=${from}&to=${from + 86400 - 1}`)
    },
    [navigate],
  )

  const navigateToHour = useCallback(
    (dateStr: string, hour: number) => {
      const y = Number.parseInt(dateStr.slice(0, 4))
      const m = Number.parseInt(dateStr.slice(4, 6)) - 1
      const d = Number.parseInt(dateStr.slice(6, 8))
      const from = Math.floor(new Date(y, m, d, hour).getTime() / 1000)
      navigate(`/logs?from=${from}&to=${from + 3600 - 1}`)
    },
    [navigate],
  )

  const navigateToWeek = useCallback(() => {
    const from = Math.floor(weekStart.getTime() / 1000)
    const end = new Date(weekStart)
    end.setDate(end.getDate() + 7)
    navigate(`/logs?from=${from}&to=${Math.floor(end.getTime() / 1000) - 1}`)
  }, [weekStart, navigate])

  const navigateToMonth = useCallback(() => {
    const from = Math.floor(monthAnchor.getTime() / 1000)
    const end = new Date(monthAnchor.getFullYear(), monthAnchor.getMonth() + 1, 1)
    navigate(`/logs?from=${from}&to=${Math.floor(end.getTime() / 1000) - 1}`)
  }, [monthAnchor, navigate])

  const navigateToYear = useCallback(() => {
    const y = today.getFullYear() + yearOffset
    const from = Math.floor(new Date(y, 0, 1).getTime() / 1000)
    navigate(`/logs?from=${from}&to=${Math.floor(new Date(y + 1, 0, 1).getTime() / 1000) - 1}`)
  }, [today, yearOffset, navigate])

  const shiftDay = useCallback(
    (delta: -1 | 1) => {
      const ds = selectedDayDateStr
      const d = new Date(
        Number.parseInt(ds.slice(0, 4)),
        Number.parseInt(ds.slice(4, 6)) - 1,
        Number.parseInt(ds.slice(6, 8)),
      )
      d.setDate(d.getDate() + delta)
      if (d > today) return
      setSelectedDateStr(toDateStr(d))
    },
    [selectedDayDateStr, today],
  )

  return {
    view: { current: view, set: setView, labels: viewLabels },
    year: { days: yearDays, label: yearLabel, offset: yearOffset, setOffset: setYearOffset },
    month: { days: monthDays, label: monthLabel, offset: monthOffset, setOffset: setMonthOffset },
    week: {
      days: weekDays,
      start: weekStart,
      label: weekLabel,
      offset: weekOffset,
      setOffset: setWeekOffset,
      dayLabels: weekdayLabels,
      dayLabelsRaw: weekdayLabelsRaw,
    },
    day: {
      dateStr: selectedDayDateStr,
      data: selectedDayData,
      displayDate: selectedDisplayDate,
      weekday: selectedDayWeekday,
      hourlyMap: dayHourlyMap,
      setDateStr: setSelectedDateStr,
    },
    navigate: {
      drillIntoDay,
      toDay: navigateToDay,
      toHour: navigateToHour,
      toWeek: navigateToWeek,
      toMonth: navigateToMonth,
      toYear: navigateToYear,
      shiftDay,
    },
    today,
  }
}

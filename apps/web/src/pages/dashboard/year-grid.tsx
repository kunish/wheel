import type { DayData, HeatmapTooltip } from "./types"
import { motion } from "motion/react"
import { getActivityLevel, LEVEL_COLORS } from "./types"

export interface YearGridProps {
  yearDays: DayData[]
  drillIntoDay: (dateStr: string) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
}

export function YearGrid({
  yearDays,
  drillIntoDay,
  handleMouseEnter,
  handleMouseLeave,
}: YearGridProps) {
  function renderCell(day: DayData | null, key: string, i: number) {
    if (!day) return <div key={key} />

    if (day.isFuture) {
      return (
        <div
          key={key}
          className="border-border/20 aspect-square rounded-[3px] border border-dashed bg-transparent opacity-30"
        />
      )
    }

    const count = (day.daily?.request_success ?? 0) + (day.daily?.request_failed ?? 0)
    const level = getActivityLevel(count)

    // Add subtle animation delay based on index for cascade effect on mount
    const delay = (i % 53) * 0.01 + Math.floor(i / 53) * 0.02

    return (
      <motion.div
        key={key}
        initial={{ opacity: 0, scale: 0.5 }}
        animate={{ opacity: 1, scale: 1 }}
        transition={{ duration: 0.3, delay, ease: "easeOut" }}
        className="relative aspect-square cursor-pointer rounded-[3px] transition-all duration-200 hover:z-10 hover:scale-[1.4] hover:shadow-md"
        style={{
          backgroundColor: level === 0 ? "var(--muted)" : LEVEL_COLORS[level],
          opacity: level === 0 ? 0.3 : 0.8 + level * 0.05,
        }}
        onClick={() => drillIntoDay(day.dateStr)}
        onMouseEnter={(e) => handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })}
        onMouseLeave={handleMouseLeave}
      >
        {/* Subtle glow for active days */}
        {level > 0 && (
          <div
            className="absolute inset-0 rounded-[3px] opacity-0 transition-opacity duration-300 hover:opacity-100"
            style={{
              boxShadow: `0 0 8px ${LEVEL_COLORS[level]}`,
              backgroundColor: "transparent",
            }}
          />
        )}
      </motion.div>
    )
  }

  return (
    <motion.div
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      transition={{ duration: 0.4 }}
      className="flex min-h-0 w-full flex-1 items-center justify-center py-4"
    >
      <div
        className="grid w-full gap-[3px] sm:gap-1"
        style={{
          gridTemplateColumns: "repeat(53, minmax(0, 1fr))",
          gridTemplateRows: "repeat(7, minmax(0, 1fr))",
          gridAutoFlow: "column",
        }}
      >
        {yearDays.map((day, i) => renderCell(day, day.dateStr, i))}
      </div>
    </motion.div>
  )
}

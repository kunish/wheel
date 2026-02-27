import type { DayData, HeatmapTooltip, ReactorGridProps } from "./types"
import { motion } from "motion/react"
import { formatCount } from "@/lib/format"
import {
  getActivityLevel,
  LEVEL_COLORS,
  levelToIntensity,
  MINI_REACTOR_ACTIVITY_R,
  MINI_REACTOR_CORE_R,
  MINI_REACTOR_CX,
  MINI_REACTOR_CY,
  MINI_REACTOR_INNER_R,
  MINI_REACTOR_OUTER_R,
} from "./types"

function MiniReactor({
  day,
  isToday,
  gearAngle,
  drillIntoDay,
  handleMouseEnter,
  handleMouseLeave,
}: {
  day: DayData
  isToday: boolean
  gearAngle: number
  drillIntoDay: (dateStr: string) => void
  handleMouseEnter: (e: React.MouseEvent, tooltip: HeatmapTooltip) => void
  handleMouseLeave: () => void
}) {
  const count = (day.daily?.request_success ?? 0) + (day.daily?.request_failed ?? 0)
  const level = day.isFuture ? 0 : getActivityLevel(count)
  const intensity = levelToIntensity(level)
  const dayNum = Number.parseInt(day.dateStr.slice(6, 8))

  const CX = MINI_REACTOR_CX
  const CY = MINI_REACTOR_CY

  // Pulse speed: faster for higher activity
  const pulseDuration = level > 0 ? `${4 - level * 0.6}s` : "5s"

  return (
    <div
      className="cursor-pointer transition-transform hover:scale-105"
      style={{ borderRadius: 8 }}
      onClick={() => drillIntoDay(day.dateStr)}
      onMouseEnter={(e) => handleMouseEnter(e, { label: day.displayDate, metrics: day.daily })}
      onMouseLeave={handleMouseLeave}
    >
      <svg viewBox="0 0 80 80" className="w-full">
        {/* ── Outer containment ring ── */}
        <circle
          cx={CX}
          cy={CY}
          r={MINI_REACTOR_OUTER_R}
          fill="none"
          stroke={isToday ? "var(--nb-lime)" : "var(--border)"}
          strokeWidth={isToday ? 1 : 0.5}
          opacity={isToday ? 0.7 : 0.3}
        />

        {/* ── Activity ring ── */}
        <circle
          cx={CX}
          cy={CY}
          r={MINI_REACTOR_ACTIVITY_R}
          fill="none"
          stroke={day.isFuture ? "var(--border)" : LEVEL_COLORS[level]}
          strokeWidth={level > 0 ? 3.5 : 1.5}
          opacity={day.isFuture ? 0.15 : level === 0 ? 0.2 : 0.8}
          strokeDasharray={level === 0 || day.isFuture ? "3 3" : "none"}
        />

        {/* ── Inner core background ── */}
        <circle
          cx={CX}
          cy={CY}
          r={MINI_REACTOR_INNER_R}
          fill="var(--card)"
          stroke="var(--border)"
          strokeWidth="0.8"
          opacity={0.8}
        />

        {/* ── Energy radial glow (core pulse) ── */}
        {level > 0 && !day.isFuture && (
          <circle
            cx={CX}
            cy={CY}
            r={MINI_REACTOR_INNER_R - 1}
            fill={`color-mix(in srgb, var(--nb-lime) ${Math.round(8 + intensity * 20)}%, transparent)`}
            style={{
              animation: `mini-reactor-pulse ${pulseDuration} ease-in-out infinite`,
              transformOrigin: `${CX}px ${CY}px`,
            }}
          />
        )}

        {/* ── Core dot ── */}
        <circle
          cx={CX}
          cy={CY}
          r={MINI_REACTOR_CORE_R}
          fill="var(--nb-lime)"
          opacity={level > 0 && !day.isFuture ? 0.4 + intensity * 0.3 : 0.08}
          style={
            level > 0 && !day.isFuture
              ? { animation: `reactor-core-pulse ${pulseDuration} ease-in-out infinite` }
              : undefined
          }
        />

        {/* ── 4 energy spokes ── */}
        {[0, 90, 180, 270].map((deg) => {
          const rad = (deg * Math.PI) / 180
          return (
            <line
              key={`spoke-${deg}`}
              x1={CX + MINI_REACTOR_INNER_R * Math.cos(rad)}
              y1={CY + MINI_REACTOR_INNER_R * Math.sin(rad)}
              x2={CX + MINI_REACTOR_ACTIVITY_R * Math.cos(rad)}
              y2={CY + MINI_REACTOR_ACTIVITY_R * Math.sin(rad)}
              stroke="var(--border)"
              strokeWidth="0.6"
              opacity={0.15}
            />
          )
        })}

        {/* ── Today: rotating dashed ring ── */}
        {isToday && (
          <circle
            cx={CX}
            cy={CY}
            r={(MINI_REACTOR_ACTIVITY_R + MINI_REACTOR_OUTER_R) / 2}
            fill="none"
            stroke="var(--nb-lime)"
            strokeWidth="0.8"
            strokeDasharray="2 4"
            opacity={0.4}
            style={{
              transformOrigin: `${CX}px ${CY}px`,
              transform: `rotate(${gearAngle}deg)`,
              transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
            }}
          />
        )}

        {/* ── Day number ── */}
        <text
          x={CX}
          y={count > 0 && !day.isFuture ? CY - 2 : CY + 1}
          textAnchor="middle"
          dominantBaseline="central"
          fill={
            isToday
              ? "var(--foreground)"
              : day.isFuture
                ? "var(--muted-foreground)"
                : "var(--foreground)"
          }
          fontSize="12"
          fontWeight="800"
          fontFamily="inherit"
          opacity={day.isFuture ? 0.25 : isToday ? 1 : 0.7}
        >
          {dayNum}
        </text>

        {/* ── Request count (below day number) ── */}
        {count > 0 && !day.isFuture && (
          <text
            x={CX}
            y={CY + 10}
            textAnchor="middle"
            dominantBaseline="central"
            fill={LEVEL_COLORS[level]}
            fontSize="7"
            fontWeight="700"
            fontFamily="inherit"
          >
            {formatCount(count).value}
          </text>
        )}
      </svg>
    </div>
  )
}

export function ReactorGrid({
  monthDays,
  weekdayLabels,
  todayStr,
  gearAngle,
  drillIntoDay,
  handleMouseEnter,
  handleMouseLeave,
}: ReactorGridProps) {
  return (
    <motion.div
      initial={{ opacity: 0, scale: 0.95 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* Weekday headers */}
      <div className="mb-1.5 grid grid-cols-7 gap-1">
        {weekdayLabels.map((label) => (
          <div key={label} className="text-muted-foreground py-0.5 text-center text-xs font-bold">
            {label}
          </div>
        ))}
      </div>

      {/* Reactor grid */}
      <div className="grid grid-cols-7 gap-1">
        {monthDays.map((day, i) => {
          // eslint-disable-next-line react/no-array-index-key -- padding slots have no unique data
          if (!day) return <div key={`pad-${i}`} />

          if (day.isFuture) {
            return (
              <motion.div
                key={day.dateStr}
                initial={{ opacity: 0, scale: 0.8 }}
                animate={{ opacity: 1, scale: 1 }}
                transition={{ duration: 0.4, delay: i * 0.015, ease: "easeOut" }}
                className="border-border/20 aspect-square rounded-lg border border-dashed opacity-30"
              >
                <svg viewBox="0 0 80 80" className="w-full">
                  <circle
                    cx={MINI_REACTOR_CX}
                    cy={MINI_REACTOR_CY}
                    r={MINI_REACTOR_OUTER_R}
                    fill="none"
                    stroke="var(--border)"
                    strokeWidth="0.5"
                    strokeDasharray="3 3"
                    opacity={0.3}
                  />
                  <text
                    x={MINI_REACTOR_CX}
                    y={MINI_REACTOR_CY + 1}
                    textAnchor="middle"
                    dominantBaseline="central"
                    fill="var(--muted-foreground)"
                    fontSize="12"
                    fontWeight="700"
                    fontFamily="inherit"
                    opacity={0.3}
                  >
                    {Number.parseInt(day.dateStr.slice(6, 8))}
                  </text>
                </svg>
              </motion.div>
            )
          }

          const isToday = day.dateStr === todayStr

          return (
            <motion.div
              key={day.dateStr}
              initial={{ opacity: 0, scale: 0.5, rotate: -10 }}
              animate={{ opacity: 1, scale: 1, rotate: 0 }}
              transition={{
                duration: 0.5,
                delay: i * 0.015,
                type: "spring",
                stiffness: 200,
                damping: 15,
              }}
            >
              <MiniReactor
                day={day}
                isToday={isToday}
                gearAngle={gearAngle}
                drillIntoDay={drillIntoDay}
                handleMouseEnter={handleMouseEnter}
                handleMouseLeave={handleMouseLeave}
              />
            </motion.div>
          )
        })}
      </div>
    </motion.div>
  )
}

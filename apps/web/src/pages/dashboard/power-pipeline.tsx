import type { PowerPipelineProps } from "./types"
import { motion } from "motion/react"
import { formatCount } from "@/lib/format"
import {
  getActivityLevel,
  LEVEL_COLORS,
  PIPE_MAX_H,
  PIPE_MIN_H,
  PIPE_SVG_H,
  PIPE_SVG_W,
} from "./types"

export function PowerPipeline({
  weekDays,
  weekdayLabels,
  todayStr,
  gearAngle,
  drillIntoDay,
  handleMouseEnter,
  handleMouseLeave,
}: PowerPipelineProps) {
  const segW = PIPE_SVG_W / 7
  const midY = PIPE_SVG_H / 2 + 8
  const spineY = midY

  // Pre-compute per-day data
  const segments = weekDays.map((day, i) => {
    const count = day.daily ? (day.daily.request_success ?? 0) + (day.daily.request_failed ?? 0) : 0
    const level = day.isFuture ? -1 : getActivityLevel(count)
    const h = day.isFuture ? PIPE_MIN_H * 0.6 : PIPE_MIN_H + (level / 4) * (PIPE_MAX_H - PIPE_MIN_H)
    const x = i * segW
    const cx = x + segW / 2
    const isToday = day.dateStr === todayStr
    const dayOfWeek = new Date(
      Number.parseInt(day.dateStr.slice(0, 4)),
      Number.parseInt(day.dateStr.slice(4, 6)) - 1,
      Number.parseInt(day.dateStr.slice(6, 8)),
    ).getDay()
    return { day, count, level, h, x, cx, isToday, dayOfWeek, i }
  })

  return (
    <motion.div
      className="relative w-full"
      initial={{ opacity: 0, y: 20 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.5, ease: [0.16, 1, 0.3, 1] }}
    >
      <svg
        viewBox={`0 0 ${PIPE_SVG_W} ${PIPE_SVG_H + 40}`}
        className="w-full"
        style={{ overflow: "visible" }}
      >
        <defs>
          <linearGradient id="pipe-energy-grad" x1="0" y1="0" x2="1" y2="0">
            <stop offset="0%" stopColor="var(--nb-lime)" stopOpacity="0.15" />
            <stop offset="50%" stopColor="var(--nb-lime)" stopOpacity="0.05" />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0.15" />
          </linearGradient>
          <filter id="pipe-glow">
            <feGaussianBlur stdDeviation="4" result="blur" />
            <feMerge>
              <feMergeNode in="blur" />
              <feMergeNode in="SourceGraphic" />
            </feMerge>
          </filter>
        </defs>

        {/* ── Background circuit texture ── */}
        {/* Horizontal grid lines */}
        {[0.25, 0.5, 0.75].map((frac) => (
          <line
            key={`hline-${frac}`}
            x1="0"
            y1={PIPE_SVG_H * frac}
            x2={PIPE_SVG_W}
            y2={PIPE_SVG_H * frac}
            stroke="var(--border)"
            strokeWidth="0.3"
            opacity={0.06}
          />
        ))}
        {/* Vertical grid lines at segment boundaries */}
        {segments.map((seg) => (
          <line
            key={`vline-${seg.i}`}
            x1={seg.x}
            y1="0"
            x2={seg.x}
            y2={PIPE_SVG_H}
            stroke="var(--border)"
            strokeWidth="0.3"
            opacity={0.06}
          />
        ))}
        {/* Grid intersection nodes */}
        {segments.map((seg) =>
          [0.25, 0.5, 0.75].map((frac) => (
            <circle
              key={`node-${seg.i}-${frac}`}
              cx={seg.x}
              cy={PIPE_SVG_H * frac}
              r="1.5"
              fill="var(--border)"
              opacity={0.08}
            />
          )),
        )}

        {/* ── Pipeline backbone / spine ── */}
        <line
          x1="0"
          y1={spineY}
          x2={PIPE_SVG_W}
          y2={spineY}
          stroke="var(--border)"
          strokeWidth="2"
          opacity={0.12}
        />
        {/* Energy trace on spine */}
        <line
          x1="0"
          y1={spineY}
          x2={PIPE_SVG_W}
          y2={spineY}
          stroke="var(--nb-lime)"
          strokeWidth="1"
          opacity={0.08}
          strokeDasharray="4 12"
          style={{
            animation: "pipeline-particle-flow 2s linear infinite",
          }}
        />

        {/* ── Pipe segments ── */}
        {segments.map((seg) => {
          const pipeW = segW * 0.55
          const rx = seg.cx - pipeW / 2
          const ry = midY - seg.h / 2

          return (
            <g key={`seg-${seg.i}`}>
              {/* Pipe shadow (neobrutalism offset shadow) */}
              {!seg.day.isFuture && seg.level > 0 && (
                <rect
                  x={rx + 2}
                  y={ry + 2}
                  width={pipeW}
                  height={seg.h}
                  rx={4}
                  fill="var(--nb-shadow)"
                  opacity={0.1}
                />
              )}

              {/* Pipe body */}
              <rect
                x={rx}
                y={ry}
                width={pipeW}
                height={seg.h}
                rx={4}
                fill={seg.day.isFuture ? "transparent" : LEVEL_COLORS[seg.level]}
                stroke={
                  seg.isToday
                    ? "var(--nb-lime)"
                    : seg.day.isFuture
                      ? "var(--border)"
                      : "var(--border)"
                }
                strokeWidth={seg.isToday ? 2.5 : seg.day.isFuture ? 1 : 1.5}
                strokeDasharray={seg.day.isFuture ? "4 3" : "none"}
                opacity={seg.day.isFuture ? 0.3 : 1}
                className={
                  seg.day.isFuture ? "" : "cursor-pointer transition-opacity hover:opacity-80"
                }
                style={
                  seg.isToday
                    ? { animation: "pipeline-segment-glow 3s ease-in-out infinite" }
                    : undefined
                }
                onClick={seg.day.isFuture ? undefined : () => drillIntoDay(seg.day.dateStr)}
                onMouseEnter={
                  seg.day.isFuture
                    ? undefined
                    : (e) =>
                        handleMouseEnter(e, {
                          label: seg.day.displayDate,
                          metrics: seg.day.daily,
                        })
                }
                onMouseLeave={seg.day.isFuture ? undefined : handleMouseLeave}
              />

              {/* Inner energy gradient overlay for active segments */}
              {!seg.day.isFuture && seg.level > 0 && (
                <rect
                  x={rx}
                  y={ry}
                  width={pipeW}
                  height={seg.h}
                  rx={4}
                  fill="url(#pipe-energy-grad)"
                  pointerEvents="none"
                />
              )}
            </g>
          )
        })}

        {/* ── Energy flow lines between adjacent active segments ── */}
        {segments.slice(0, -1).map((seg, i) => {
          const next = segments[i + 1]
          if (seg.level <= 0 || next.level <= 0) return null
          const pipeW = segW * 0.55
          return (
            <line
              key={`flow-${i}`}
              x1={seg.cx + pipeW / 2}
              y1={spineY}
              x2={next.cx - pipeW / 2}
              y2={spineY}
              stroke="var(--nb-lime)"
              strokeWidth="2"
              opacity={0.25}
              strokeDasharray="4 4"
              style={{
                animation: "energy-channel-flow 1.5s linear infinite",
              }}
            />
          )
        })}

        {/* ── Connection nodes (gear circles between segments) ── */}
        {segments.slice(0, -1).map((seg, i) => {
          const nodeX = seg.x + segW
          return (
            <g key={`conn-${i}`}>
              <circle
                cx={nodeX}
                cy={spineY}
                r="5"
                fill="var(--card)"
                stroke="var(--border)"
                strokeWidth="1.5"
              />
              {/* Mini gear teeth (4 marks) */}
              {[0, 90, 180, 270].map((deg) => {
                const rad = ((deg + gearAngle) * Math.PI) / 180
                return (
                  <line
                    key={`gear-mark-${i}-${deg}`}
                    x1={nodeX + 3.5 * Math.cos(rad)}
                    y1={spineY + 3.5 * Math.sin(rad)}
                    x2={nodeX + 6.5 * Math.cos(rad)}
                    y2={spineY + 6.5 * Math.sin(rad)}
                    stroke="var(--border)"
                    strokeWidth="1.2"
                    opacity={0.3}
                    style={{
                      transformOrigin: `${nodeX}px ${spineY}px`,
                      transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
                    }}
                  />
                )
              })}
              <circle cx={nodeX} cy={spineY} r="2" fill="var(--border)" opacity={0.15} />
            </g>
          )
        })}

        {/* ── Labels row: weekday + date + count ── */}
        {segments.map((seg) => {
          const labelY = midY + seg.h / 2 + 16
          return (
            <g key={`label-${seg.i}`}>
              <text
                x={seg.cx}
                y={labelY}
                textAnchor="middle"
                fill={seg.isToday ? "var(--foreground)" : "var(--muted-foreground)"}
                fontSize="11"
                fontWeight={seg.isToday ? "900" : "700"}
                fontFamily="inherit"
              >
                {weekdayLabels[seg.dayOfWeek]}
              </text>
              <text
                x={seg.cx}
                y={labelY + 13}
                textAnchor="middle"
                fill="var(--muted-foreground)"
                fontSize="9"
                fontWeight="500"
                fontFamily="inherit"
                opacity={0.7}
              >
                {seg.day.dateStr.slice(4, 6)}/{seg.day.dateStr.slice(6, 8)}
              </text>
              {!seg.day.isFuture && seg.count > 0 && (
                <text
                  x={seg.cx}
                  y={midY - seg.h / 2 - 8}
                  textAnchor="middle"
                  fill="var(--foreground)"
                  fontSize="10"
                  fontWeight="800"
                  fontFamily="inherit"
                >
                  {formatCount(seg.count).value}
                </text>
              )}
            </g>
          )
        })}
      </svg>
    </motion.div>
  )
}

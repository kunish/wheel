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
      transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
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
          <linearGradient id="pipe-glow-grad" x1="0" y1="0" x2="0" y2="1">
            <stop offset="0%" stopColor="var(--nb-lime)" stopOpacity="0.4" />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0.0" />
          </linearGradient>
          <filter id="pipe-glow" x="-20%" y="-20%" width="140%" height="140%">
            <feGaussianBlur stdDeviation="6" result="blur" />
            <feComposite in="SourceGraphic" in2="blur" operator="over" />
          </filter>
        </defs>

        {/* ── Background circuit texture ── */}
        <motion.g
          initial={{ opacity: 0 }}
          animate={{ opacity: 1 }}
          transition={{ duration: 0.8, delay: 0.2 }}
        >
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
              opacity={0.08}
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
              opacity={0.08}
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
                opacity={0.12}
              />
            )),
          )}
        </motion.g>

        {/* ── Pipeline backbone / spine ── */}
        <motion.g
          initial={{ pathLength: 0, opacity: 0 }}
          animate={{ pathLength: 1, opacity: 1 }}
          transition={{ duration: 1, ease: "easeInOut" }}
        >
          <line
            x1="0"
            y1={spineY}
            x2={PIPE_SVG_W}
            y2={spineY}
            stroke="var(--border)"
            strokeWidth="3"
            opacity={0.15}
            strokeLinecap="round"
          />
        </motion.g>

        {/* Energy trace on spine */}
        <line
          x1="0"
          y1={spineY}
          x2={PIPE_SVG_W}
          y2={spineY}
          stroke="var(--nb-lime)"
          strokeWidth="1.5"
          opacity={0.15}
          strokeDasharray="4 12"
          strokeLinecap="round"
          style={{
            animation: "pipeline-particle-flow 1.5s linear infinite",
          }}
        />

        {/* ── Pipe segments ── */}
        {segments.map((seg) => {
          const pipeW = segW * 0.55
          const rx = seg.cx - pipeW / 2
          const ry = midY - seg.h / 2

          return (
            <g key={`seg-${seg.i}`}>
              {/* Pipe drop shadow */}
              {!seg.day.isFuture && seg.level > 0 && (
                <motion.rect
                  initial={{ height: 0, y: midY }}
                  animate={{ height: seg.h, y: ry + 4 }}
                  transition={{
                    duration: 0.5,
                    delay: 0.1 + seg.i * 0.05,
                    type: "spring",
                    stiffness: 100,
                  }}
                  x={rx + 4}
                  width={pipeW}
                  rx={6}
                  fill="rgb(0 0 0 / 0.15)"
                  opacity={0.1}
                  filter="url(#pipe-glow)"
                />
              )}

              {/* Pipe body */}
              <motion.rect
                initial={{ height: PIPE_MIN_H * 0.6, y: midY - (PIPE_MIN_H * 0.6) / 2 }}
                animate={{ height: seg.h, y: ry }}
                transition={{ duration: 0.6, delay: seg.i * 0.05, type: "spring", bounce: 0.3 }}
                x={rx}
                width={pipeW}
                rx={6}
                fill={seg.day.isFuture ? "transparent" : LEVEL_COLORS[seg.level]}
                stroke={
                  seg.isToday
                    ? "var(--nb-lime)"
                    : seg.day.isFuture
                      ? "var(--border)"
                      : "transparent"
                }
                strokeWidth={seg.isToday ? 2 : seg.day.isFuture ? 1 : 0}
                strokeDasharray={seg.day.isFuture ? "4 4" : "none"}
                opacity={seg.day.isFuture ? 0.2 : 0.9}
                className={
                  seg.day.isFuture
                    ? ""
                    : "cursor-pointer transition-all duration-300 hover:opacity-100 hover:brightness-110"
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
                <motion.rect
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.5, delay: 0.3 + seg.i * 0.05 }}
                  x={rx}
                  y={ry}
                  width={pipeW}
                  height={seg.h}
                  rx={6}
                  fill="url(#pipe-energy-grad)"
                  pointerEvents="none"
                />
              )}

              {/* Highlight on top of the pipe for 3D effect */}
              {!seg.day.isFuture && seg.level > 0 && (
                <motion.rect
                  initial={{ opacity: 0 }}
                  animate={{ opacity: 1 }}
                  transition={{ duration: 0.5, delay: 0.4 + seg.i * 0.05 }}
                  x={rx + 2}
                  y={ry + 2}
                  width={pipeW - 4}
                  height={seg.h * 0.2}
                  rx={4}
                  fill="url(#pipe-glow-grad)"
                  pointerEvents="none"
                />
              )}
            </g>
          )
        })}

        {/* ── Energy flow lines between adjacent active segments ── */}
        {segments.slice(0, -1).map((seg) => {
          const next = segments[seg.i + 1]
          if (seg.level <= 0 || next.level <= 0) return null
          const pipeW = segW * 0.55
          return (
            <motion.line
              key={`flow-${seg.i}`}
              initial={{ pathLength: 0, opacity: 0 }}
              animate={{ pathLength: 1, opacity: 0.4 }}
              transition={{ duration: 0.8, delay: 0.5 + seg.i * 0.1 }}
              x1={seg.cx + pipeW / 2}
              y1={spineY}
              x2={next.cx - pipeW / 2}
              y2={spineY}
              stroke="var(--nb-lime)"
              strokeWidth="2"
              strokeDasharray="4 4"
              style={{
                animation: "energy-channel-flow 1.2s linear infinite",
              }}
            />
          )
        })}

        {/* ── Connection nodes (gear circles between segments) ── */}
        {segments.slice(0, -1).map((seg) => {
          const nodeX = seg.x + segW
          return (
            <motion.g
              key={`conn-${seg.i}`}
              initial={{ scale: 0, opacity: 0 }}
              animate={{ scale: 1, opacity: 1 }}
              transition={{
                duration: 0.4,
                delay: 0.3 + seg.i * 0.05,
                type: "spring",
                stiffness: 200,
              }}
            >
              <circle
                cx={nodeX}
                cy={spineY}
                r="6"
                fill="var(--card)"
                stroke="var(--border)"
                strokeWidth="2"
              />
              {/* Mini gear teeth (4 marks) */}
              {[0, 90, 180, 270].map((deg) => {
                const rad = ((deg + gearAngle) * Math.PI) / 180
                return (
                  <line
                    key={`gear-mark-${seg.i}-${deg}`}
                    x1={nodeX + 4 * Math.cos(rad)}
                    y1={spineY + 4 * Math.sin(rad)}
                    x2={nodeX + 8 * Math.cos(rad)}
                    y2={spineY + 8 * Math.sin(rad)}
                    stroke="var(--border)"
                    strokeWidth="1.5"
                    opacity={0.4}
                    style={{
                      transformOrigin: `${nodeX}px ${spineY}px`,
                      transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
                    }}
                  />
                )
              })}
              <circle cx={nodeX} cy={spineY} r="2.5" fill="var(--nb-lime)" opacity={0.5} />
            </motion.g>
          )
        })}

        {/* ── Labels row: weekday + date + count ── */}
        {segments.map((seg) => {
          const labelY = midY + seg.h / 2 + 18
          return (
            <motion.g
              key={`label-${seg.i}`}
              initial={{ opacity: 0, y: -10 }}
              animate={{ opacity: 1, y: 0 }}
              transition={{ duration: 0.5, delay: 0.4 + seg.i * 0.05 }}
            >
              <text
                x={seg.cx}
                y={labelY}
                textAnchor="middle"
                fill={seg.isToday ? "var(--foreground)" : "var(--muted-foreground)"}
                fontSize="12"
                fontWeight={seg.isToday ? "800" : "600"}
                fontFamily="inherit"
              >
                {weekdayLabels[seg.dayOfWeek]}
              </text>
              <text
                x={seg.cx}
                y={labelY + 14}
                textAnchor="middle"
                fill="var(--muted-foreground)"
                fontSize="10"
                fontWeight="500"
                fontFamily="inherit"
                opacity={0.8}
              >
                {seg.day.dateStr.slice(4, 6)}/{seg.day.dateStr.slice(6, 8)}
              </text>
              {!seg.day.isFuture && seg.count > 0 && (
                <text
                  x={seg.cx}
                  y={midY - seg.h / 2 - 10}
                  textAnchor="middle"
                  fill="var(--foreground)"
                  fontSize="11"
                  fontWeight="800"
                  fontFamily="inherit"
                >
                  {formatCount(seg.count).value}
                </text>
              )}
            </motion.g>
          )
        })}
      </svg>
    </motion.div>
  )
}

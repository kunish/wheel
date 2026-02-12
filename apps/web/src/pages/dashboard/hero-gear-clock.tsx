import type { HeroGearClockProps } from "./types"
import { motion } from "motion/react"
import {
  GEAR_AM_INNER,
  GEAR_AM_OUTER,
  GEAR_ARC_SPAN,
  GEAR_BASE,
  GEAR_CX,
  GEAR_CY,
  GEAR_GAP,
  GEAR_HUB_R,
  GEAR_OUTER_R,
  GEAR_PM_INNER,
  GEAR_PM_OUTER,
  GEAR_RING_R,
  GEAR_TOOTH_H,
  gearArcPath,
  getActivityLevel,
  LEVEL_COLORS,
} from "./types"

export function HeroGearClock({
  dayHourlyMap,
  isToday,
  nowHour,
  now,
  selectedDayDateStr,
  selectedDisplayDate,
  gearAngle,
  navigateToHour,
  handleMouseEnter,
  handleMouseLeave,
  children,
}: HeroGearClockProps) {
  const toRad = (deg: number) => (deg * Math.PI) / 180

  // Count active hours for reactor energy intensity
  const activeHours = Array.from({ length: 24 }, (_, h) => {
    const hourly = dayHourlyMap.get(h)
    return hourly ? (hourly.request_success ?? 0) + (hourly.request_failed ?? 0) : 0
  }).filter((c) => c > 0).length

  const energyIntensity = Math.min(1, activeHours / 12)

  return (
    <motion.div
      className="relative flex items-center justify-center py-2"
      initial={{ opacity: 0, scale: 0.8 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.6, ease: [0.16, 1, 0.3, 1] }}
    >
      {/* ── Arc Reactor background glow ── */}
      <div
        className="pointer-events-none absolute rounded-full"
        style={{
          width: "85%",
          height: "85%",
          background: `radial-gradient(circle, color-mix(in srgb, var(--nb-lime) ${Math.round(8 + energyIntensity * 10)}%, transparent) 0%, transparent 70%)`,
          animation: "reactor-core-pulse 3s ease-in-out infinite",
        }}
      />

      {/* ── Decorative outer reactor rings — slow rotating, breathing ── */}
      <svg
        viewBox="0 0 400 400"
        className="pointer-events-none absolute w-full"
        style={{ animation: "reactor-ring-breathe 4s ease-in-out infinite" }}
      >
        {/* Outermost energy ring */}
        <circle
          cx="200"
          cy="200"
          r="198"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.5"
          opacity={0.3}
        />
        {/* Outer dashed containment ring */}
        <circle
          cx="200"
          cy="200"
          r="194"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.3"
          strokeDasharray="2 6"
          opacity={0.2}
          style={{ animation: "reactor-spin-slow 60s linear infinite" }}
        />
      </svg>

      {/* ── Counter-rotating decorative gear (behind main) ── */}
      <svg
        viewBox="0 0 400 400"
        className="pointer-events-none absolute w-full opacity-[0.04]"
        style={{
          transform: `rotate(${-gearAngle * 0.5}deg)`,
          transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
        }}
      >
        {Array.from({ length: 32 }, (_, i) => {
          const angle = i * (360 / 32)
          const rad = toRad(angle)
          const perp = rad + Math.PI / 2
          const hw = 5
          const iR = 185
          const oR = 198
          return (
            <path
              key={i}
              d={`M ${200 + iR * Math.cos(rad) + hw * Math.cos(perp)} ${200 + iR * Math.sin(rad) + hw * Math.sin(perp)} L ${200 + oR * Math.cos(rad) + (hw - 1) * Math.cos(perp)} ${200 + oR * Math.sin(rad) + (hw - 1) * Math.sin(perp)} L ${200 + oR * Math.cos(rad) - (hw - 1) * Math.cos(perp)} ${200 + oR * Math.sin(rad) - (hw - 1) * Math.sin(perp)} L ${200 + iR * Math.cos(rad) - hw * Math.cos(perp)} ${200 + iR * Math.sin(rad) - hw * Math.sin(perp)} Z`}
              fill="var(--foreground)"
            />
          )
        })}
        <circle cx="200" cy="200" r="185" fill="none" stroke="var(--foreground)" strokeWidth="3" />
      </svg>

      {/* ── Main gear SVG ── */}
      <svg viewBox="-15 -15 430 430" className="relative w-full">
        <defs>
          {/* Reactor core gradient for hub */}
          <radialGradient id="reactor-core-grad" cx="50%" cy="50%" r="50%">
            <stop
              offset="0%"
              stopColor="var(--nb-lime)"
              stopOpacity={0.12 + energyIntensity * 0.08}
            />
            <stop offset="60%" stopColor="var(--nb-lime)" stopOpacity={0.04} />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0" />
          </radialGradient>
          {/* Energy channel gradient for spokes */}
          <linearGradient id="spoke-energy" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="var(--nb-lime)" stopOpacity="0.3" />
            <stop offset="50%" stopColor="var(--nb-lime)" stopOpacity="0.08" />
            <stop offset="100%" stopColor="var(--nb-lime)" stopOpacity="0.3" />
          </linearGradient>
        </defs>

        {/* ── Rotating gear teeth ring ── */}
        <g
          style={{
            transformOrigin: `${GEAR_CX}px ${GEAR_CY}px`,
            filter: "drop-shadow(2px 2px 0 color-mix(in srgb, var(--nb-shadow) 25%, transparent))",
            transform: `rotate(${gearAngle}deg)`,
            transition: "transform 0.6s cubic-bezier(0.34, 1.56, 0.64, 1)",
          }}
        >
          {Array.from({ length: 24 }, (_, i) => {
            const seg = Math.floor(i / 2)
            const sub = i % 2
            const segCenter = seg * 30 - 90
            const toothAngle = segCenter + (sub === 0 ? -7 : 7)
            const hw = 4.5
            const mid = toRad(toothAngle)
            const perp = mid + Math.PI / 2
            const ix1 = GEAR_CX + GEAR_RING_R * Math.cos(mid) + hw * Math.cos(perp)
            const iy1 = GEAR_CY + GEAR_RING_R * Math.sin(mid) + hw * Math.sin(perp)
            const ix2 = GEAR_CX + GEAR_RING_R * Math.cos(mid) - hw * Math.cos(perp)
            const iy2 = GEAR_CY + GEAR_RING_R * Math.sin(mid) - hw * Math.sin(perp)
            const ox1 = GEAR_CX + GEAR_OUTER_R * Math.cos(mid) + (hw - 1) * Math.cos(perp)
            const oy1 = GEAR_CY + GEAR_OUTER_R * Math.sin(mid) + (hw - 1) * Math.sin(perp)
            const ox2 = GEAR_CX + GEAR_OUTER_R * Math.cos(mid) - (hw - 1) * Math.cos(perp)
            const oy2 = GEAR_CY + GEAR_OUTER_R * Math.sin(mid) - (hw - 1) * Math.sin(perp)
            return (
              <path
                key={`tooth-${i}`}
                d={`M ${ix1} ${iy1} L ${ox1} ${oy1} L ${ox2} ${oy2} L ${ix2} ${iy2} Z`}
                fill="var(--primary)"
                opacity={0.25}
              />
            )
          })}
          {/* Outer gear ring */}
          <circle
            cx={GEAR_CX}
            cy={GEAR_CY}
            r={GEAR_RING_R}
            fill="none"
            stroke="var(--border)"
            strokeWidth="2.5"
          />
        </g>

        {/* ── PM ring (outer): hours 12-23 ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const h = i + 12
          const isFutureHour = isToday && h > nowHour
          const hourly = dayHourlyMap.get(h)
          const hCount = isFutureHour
            ? 0
            : (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
          const hLevel = isFutureHour ? -1 : getActivityLevel(hCount)
          const startDeg = i * 30 + GEAR_BASE + GEAR_GAP
          const isCurrentHour = isToday && h === nowHour

          return (
            <path
              key={`pm-${h}`}
              d={gearArcPath(startDeg, GEAR_ARC_SPAN, GEAR_PM_INNER, GEAR_PM_OUTER)}
              fill={hLevel === -1 ? "transparent" : LEVEL_COLORS[hLevel]}
              stroke={isCurrentHour ? "var(--nb-lime)" : hLevel === -1 ? "var(--border)" : "none"}
              strokeWidth={isCurrentHour ? "2" : hLevel === -1 ? "0.5" : "0"}
              strokeDasharray={hLevel === -1 && !isCurrentHour ? "3 2" : "none"}
              opacity={hLevel === -1 ? 0.3 : 1}
              className="cursor-pointer transition-all hover:opacity-80"
              style={isCurrentHour ? { filter: "drop-shadow(0 0 6px var(--nb-lime))" } : undefined}
              onClick={() => navigateToHour(selectedDayDateStr, h)}
              onMouseEnter={(e) =>
                handleMouseEnter(e, {
                  label: `${selectedDisplayDate} ${h.toString().padStart(2, "0")}:00`,
                  metrics: hourly ?? null,
                })
              }
              onMouseLeave={handleMouseLeave}
            />
          )
        })}

        {/* ── Divider ring between AM / PM — reactor containment ring ── */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={131}
          fill="none"
          stroke="var(--border)"
          strokeWidth="1.5"
          pointerEvents="none"
        />
        {/* Energy trace on divider */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={131}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.5"
          opacity={0.15}
          strokeDasharray="8 20"
          pointerEvents="none"
          style={{
            transformOrigin: `${GEAR_CX}px ${GEAR_CY}px`,
            animation: "reactor-spin-slow 30s linear infinite",
          }}
        />

        {/* ── AM ring (inner): hours 0-11 ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const h = i
          const isFutureHour = isToday && h > nowHour
          const hourly = dayHourlyMap.get(h)
          const hCount = isFutureHour
            ? 0
            : (hourly?.request_success ?? 0) + (hourly?.request_failed ?? 0)
          const hLevel = isFutureHour ? -1 : getActivityLevel(hCount)
          const startDeg = i * 30 + GEAR_BASE + GEAR_GAP
          const isCurrentHour = isToday && h === nowHour

          return (
            <path
              key={`am-${h}`}
              d={gearArcPath(startDeg, GEAR_ARC_SPAN, GEAR_AM_INNER, GEAR_AM_OUTER)}
              fill={hLevel === -1 ? "transparent" : LEVEL_COLORS[hLevel]}
              stroke={isCurrentHour ? "var(--nb-lime)" : hLevel === -1 ? "var(--border)" : "none"}
              strokeWidth={isCurrentHour ? "2" : hLevel === -1 ? "0.5" : "0"}
              strokeDasharray={hLevel === -1 && !isCurrentHour ? "3 2" : "none"}
              opacity={hLevel === -1 ? 0.3 : 1}
              className="cursor-pointer transition-all hover:opacity-80"
              style={isCurrentHour ? { filter: "drop-shadow(0 0 6px var(--nb-lime))" } : undefined}
              onClick={() => navigateToHour(selectedDayDateStr, h)}
              onMouseEnter={(e) =>
                handleMouseEnter(e, {
                  label: `${selectedDisplayDate} ${h.toString().padStart(2, "0")}:00`,
                  metrics: hourly ?? null,
                })
              }
              onMouseLeave={handleMouseLeave}
            />
          )
        })}

        {/* ── Reactor core hub ── */}
        {/* Outer glow ring around hub */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R + 2}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.8"
          opacity={0.2}
          pointerEvents="none"
          style={{ animation: "reactor-core-pulse 3s ease-in-out infinite" }}
        />
        {/* Hub fill */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R}
          fill="var(--card)"
          stroke="var(--border)"
          strokeWidth="2.5"
          pointerEvents="none"
        />
        {/* Reactor energy fill inside hub */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R - 1}
          fill="url(#reactor-core-grad)"
          pointerEvents="none"
          style={{ animation: "reactor-core-pulse 3s ease-in-out infinite" }}
        />

        {/* ── Energy channel spokes ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const angle = toRad(i * 30 + GEAR_BASE + 15)
          const x1 = GEAR_CX + GEAR_HUB_R * Math.cos(angle)
          const y1 = GEAR_CY + GEAR_HUB_R * Math.sin(angle)
          const x2 = GEAR_CX + GEAR_RING_R * Math.cos(angle)
          const y2 = GEAR_CY + GEAR_RING_R * Math.sin(angle)
          return (
            <g key={`spoke-${i}`} pointerEvents="none">
              {/* Energy glow line */}
              <line
                x1={x1}
                y1={y1}
                x2={x2}
                y2={y2}
                stroke="var(--nb-lime)"
                strokeWidth="2.5"
                opacity={0.06}
                strokeLinecap="round"
              />
              {/* Structural spoke */}
              <line
                x1={x1}
                y1={y1}
                x2={x2}
                y2={y2}
                stroke="var(--border)"
                strokeWidth="1.2"
                opacity={0.1}
              />
            </g>
          )
        })}

        {/* ── Inner reactor rings (decorative concentric circles) ── */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r={GEAR_HUB_R - 6}
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.4"
          opacity={0.12}
          pointerEvents="none"
        />
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r="12"
          fill="none"
          stroke="var(--nb-lime)"
          strokeWidth="0.6"
          opacity={0.2}
          pointerEvents="none"
          style={{ animation: "reactor-core-pulse 2.5s ease-in-out infinite" }}
        />
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r="8"
          fill="var(--border)"
          opacity={0.1}
          pointerEvents="none"
        />
        {/* Core dot — energy center */}
        <circle
          cx={GEAR_CX}
          cy={GEAR_CY}
          r="3"
          fill="var(--nb-lime)"
          opacity={0.3}
          pointerEvents="none"
          style={{ animation: "reactor-core-pulse 2s ease-in-out infinite" }}
        />

        {/* ── Clock hour labels ── */}
        {Array.from({ length: 12 }, (_, i) => {
          const displayHour = i === 0 ? 12 : i
          const midAngle = i * 30 - 90
          const labelR = GEAR_RING_R + GEAR_TOOTH_H / 2 + 12
          const x = GEAR_CX + labelR * Math.cos(toRad(midAngle))
          const y = GEAR_CY + labelR * Math.sin(toRad(midAngle))
          const isCardinal = i % 3 === 0
          return (
            <text
              key={`label-${i}`}
              x={x}
              y={y}
              textAnchor="middle"
              dominantBaseline="central"
              fill="var(--foreground)"
              fontSize={isCardinal ? "13" : "9"}
              fontWeight={isCardinal ? "900" : "700"}
              fontFamily="inherit"
              opacity={isCardinal ? 1 : 0.5}
              pointerEvents="none"
            >
              {displayHour}
            </text>
          )
        })}

        {/* ── AM / PM labels ── */}
        <text
          x={GEAR_CX}
          y={GEAR_CY - (GEAR_PM_INNER + GEAR_PM_OUTER) / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill="var(--muted-foreground)"
          fontSize="7"
          fontWeight="800"
          fontFamily="inherit"
          letterSpacing="2"
          opacity={0.45}
          pointerEvents="none"
        >
          PM
        </text>
        <text
          x={GEAR_CX}
          y={GEAR_CY - (GEAR_AM_INNER + GEAR_AM_OUTER) / 2}
          textAnchor="middle"
          dominantBaseline="central"
          fill="var(--muted-foreground)"
          fontSize="7"
          fontWeight="800"
          fontFamily="inherit"
          letterSpacing="2"
          opacity={0.45}
          pointerEvents="none"
        >
          AM
        </text>

        {/* ── Current hour hand (today only) ── */}
        {isToday &&
          (() => {
            const minuteFraction = now.getMinutes() / 60
            const clockPos = (nowHour % 12) + minuteFraction
            const handAngle = clockPos * 30 - 90
            const isPM = nowHour >= 12
            const handLen = isPM ? GEAR_PM_OUTER : GEAR_AM_OUTER
            const hx = GEAR_CX + handLen * Math.cos(toRad(handAngle))
            const hy = GEAR_CY + handLen * Math.sin(toRad(handAngle))
            return (
              <g pointerEvents="none">
                {/* Hand glow */}
                <line
                  x1={GEAR_CX}
                  y1={GEAR_CY}
                  x2={hx}
                  y2={hy}
                  stroke="var(--nb-lime)"
                  strokeWidth="6"
                  strokeLinecap="round"
                  opacity={0.15}
                />
                {/* Hand line */}
                <line
                  x1={GEAR_CX}
                  y1={GEAR_CY}
                  x2={hx}
                  y2={hy}
                  stroke="var(--destructive)"
                  strokeWidth="3"
                  strokeLinecap="round"
                />
                <circle cx={GEAR_CX} cy={GEAR_CY} r="5" fill="var(--destructive)" />
                <circle cx={hx} cy={hy} r="3.5" fill="var(--destructive)" />
              </g>
            )
          })()}

        {/* ── Center stats removed — rendered via HTML overlay below ── */}
      </svg>

      {/* ── HTML overlay for hub content (children) ── */}
      {children && (
        <div className="pointer-events-none absolute inset-0 flex items-center justify-center">
          <div className="pointer-events-auto flex flex-col items-center justify-center">
            {children}
          </div>
        </div>
      )}
    </motion.div>
  )
}

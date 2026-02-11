import { cn } from "@/lib/utils"

const SIZE_MAP = { sm: 16, md: 24, lg: 40 } as const
const SPEED_MAP = {
  slow: "animate-gear-spin-slow",
  normal: "animate-gear-spin",
  fast: "[animation-duration:1s]",
} as const

interface GearSpinnerProps {
  size?: "sm" | "md" | "lg"
  speed?: "slow" | "normal" | "fast"
  className?: string
}

/**
 * SVG gear spinner with 8 trapezoidal teeth, outer ring, inner hub,
 * 4 spokes, and center dot. Inherits `currentColor`.
 */
export function GearSpinner({ size = "md", speed = "normal", className }: GearSpinnerProps) {
  const px = SIZE_MAP[size]
  const speedClass =
    speed === "fast" ? "animate-gear-spin [animation-duration:1s]" : SPEED_MAP[speed]

  return (
    <svg
      viewBox="0 0 40 40"
      width={px}
      height={px}
      fill="none"
      stroke="currentColor"
      className={cn(speedClass, className)}
      style={{ willChange: "transform" }}
    >
      <GearTeeth />
      {/* Outer ring */}
      <circle cx="20" cy="20" r="14" strokeWidth="1.5" />
      {/* Inner hub */}
      <circle cx="20" cy="20" r="6" strokeWidth="1.5" />
      {/* 4 spokes */}
      {[0, 90, 180, 270].map((deg) => {
        const rad = (deg * Math.PI) / 180
        return (
          <line
            key={deg}
            x1={20 + 6 * Math.cos(rad)}
            y1={20 + 6 * Math.sin(rad)}
            x2={20 + 14 * Math.cos(rad)}
            y2={20 + 14 * Math.sin(rad)}
            strokeWidth="1.5"
          />
        )
      })}
      {/* Center dot */}
      <circle cx="20" cy="20" r="2" fill="currentColor" stroke="none" />
    </svg>
  )
}

/** 8 trapezoidal gear teeth at 45° intervals */
function GearTeeth() {
  return (
    <>
      {Array.from({ length: 8 }, (_, i) => {
        const angle = i * 45
        const rad = (angle * Math.PI) / 180
        const innerR = 14
        const outerR = 18
        const halfW = 3.2
        const cos = Math.cos(rad)
        const sin = Math.sin(rad)
        const px = -sin
        const py = cos

        const ix1 = 20 + innerR * cos + halfW * px
        const iy1 = 20 + innerR * sin + halfW * py
        const ix2 = 20 + innerR * cos - halfW * px
        const iy2 = 20 + innerR * sin - halfW * py
        const ox1 = 20 + outerR * cos + (halfW - 0.8) * px
        const oy1 = 20 + outerR * sin + (halfW - 0.8) * py
        const ox2 = 20 + outerR * cos - (halfW - 0.8) * px
        const oy2 = 20 + outerR * sin - (halfW - 0.8) * py

        return (
          <path
            key={i}
            d={`M ${ix1} ${iy1} L ${ox1} ${oy1} L ${ox2} ${oy2} L ${ix2} ${iy2} Z`}
            fill="currentColor"
            stroke="none"
          />
        )
      })}
    </>
  )
}

/**
 * Outline-only gear SVG for decorative backgrounds.
 * No animation — just stroke, inherits `currentColor`.
 */
export function GearOutline({ size = 40, className }: { size?: number; className?: string }) {
  return (
    <svg
      viewBox="0 0 40 40"
      width={size}
      height={size}
      fill="none"
      stroke="currentColor"
      className={className}
    >
      {/* Teeth outlines */}
      {Array.from({ length: 8 }, (_, i) => {
        const angle = i * 45
        const rad = (angle * Math.PI) / 180
        const innerR = 14
        const outerR = 18
        const halfW = 3.2
        const cos = Math.cos(rad)
        const sin = Math.sin(rad)
        const px = -sin
        const py = cos

        const ix1 = 20 + innerR * cos + halfW * px
        const iy1 = 20 + innerR * sin + halfW * py
        const ix2 = 20 + innerR * cos - halfW * px
        const iy2 = 20 + innerR * sin - halfW * py
        const ox1 = 20 + outerR * cos + (halfW - 0.8) * px
        const oy1 = 20 + outerR * sin + (halfW - 0.8) * py
        const ox2 = 20 + outerR * cos - (halfW - 0.8) * px
        const oy2 = 20 + outerR * sin - (halfW - 0.8) * py

        return (
          <path
            key={i}
            d={`M ${ix1} ${iy1} L ${ox1} ${oy1} L ${ox2} ${oy2} L ${ix2} ${iy2} Z`}
            strokeWidth="1"
          />
        )
      })}
      <circle cx="20" cy="20" r="14" strokeWidth="1" />
      <circle cx="20" cy="20" r="6" strokeWidth="1" />
      {[0, 90, 180, 270].map((deg) => {
        const rad = (deg * Math.PI) / 180
        return (
          <line
            key={deg}
            x1={20 + 6 * Math.cos(rad)}
            y1={20 + 6 * Math.sin(rad)}
            x2={20 + 14 * Math.cos(rad)}
            y2={20 + 14 * Math.sin(rad)}
            strokeWidth="1"
          />
        )
      })}
      <circle cx="20" cy="20" r="2" strokeWidth="1" />
    </svg>
  )
}

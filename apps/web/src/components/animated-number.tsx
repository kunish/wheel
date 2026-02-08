"use client"

import { useMotionValue, useMotionValueEvent, useSpring } from "motion/react"
import { useEffect, useRef } from "react"

interface AnimatedNumberProps {
  value: number
  formatter?: (value: number) => string
  className?: string
}

export function AnimatedNumber({ value, formatter, className }: AnimatedNumberProps) {
  const ref = useRef<HTMLSpanElement>(null)
  const motionValue = useMotionValue(value)
  const spring = useSpring(motionValue, { stiffness: 100, damping: 20 })

  useEffect(() => {
    motionValue.set(value)
  }, [motionValue, value])

  useMotionValueEvent(spring, "change", (latest) => {
    if (ref.current) {
      ref.current.textContent = formatter ? formatter(latest) : String(Math.round(latest))
    }
  })

  return (
    <span ref={ref} className={className} suppressHydrationWarning>
      {formatter ? formatter(value) : String(Math.round(value))}
    </span>
  )
}

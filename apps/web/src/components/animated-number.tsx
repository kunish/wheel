import { useEffect, useRef } from "react"

interface AnimatedNumberProps {
  value: number
  formatter?: (value: number) => string
  className?: string
}

const DURATION = 400 // ms

function lerp(from: number, to: number, t: number) {
  return from + (to - from) * t
}

function easeOut(t: number) {
  return 1 - (1 - t) ** 3
}

export function AnimatedNumber({ value, formatter, className }: AnimatedNumberProps) {
  const ref = useRef<HTMLSpanElement>(null)
  const prevValueRef = useRef(value)
  const rafIdRef = useRef(0)

  useEffect(() => {
    const from = prevValueRef.current
    const to = value
    prevValueRef.current = value

    if (from === to) return

    const start = performance.now()

    function tick(now: number) {
      const elapsed = now - start
      const progress = Math.min(elapsed / DURATION, 1)
      const current = lerp(from, to, easeOut(progress))

      if (ref.current) {
        ref.current.textContent = formatter ? formatter(current) : String(Math.round(current))
      }

      if (progress < 1) {
        rafIdRef.current = requestAnimationFrame(tick)
      }
    }

    rafIdRef.current = requestAnimationFrame(tick)
    return () => cancelAnimationFrame(rafIdRef.current)
  }, [value, formatter])

  return (
    <span ref={ref} className={className}>
      {formatter ? formatter(value) : String(Math.round(value))}
    </span>
  )
}

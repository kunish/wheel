import { useLayoutEffect, useRef } from "react"

let lastPointer = { x: 0, y: 0 }
let initialized = false

function ensureListener() {
  if (initialized || typeof document === "undefined") return
  initialized = true
  document.addEventListener(
    "pointerdown",
    (e) => {
      lastPointer = { x: e.clientX, y: e.clientY }
    },
    true,
  )
}

export function usePointerOrigin<T extends HTMLElement = HTMLDivElement>() {
  ensureListener()
  const ref = useRef<T>(null)

  useLayoutEffect(() => {
    const el = ref.current
    if (!el) return
    const rect = el.getBoundingClientRect()
    el.style.transformOrigin = `${lastPointer.x - rect.left}px ${lastPointer.y - rect.top}px`
  })

  return ref
}

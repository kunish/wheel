"use client"

import { useRouter } from "next/navigation"
import { useEffect } from "react"

export default function GroupsRedirect() {
  const router = useRouter()
  useEffect(() => {
    router.replace("/channels")
  }, [router])
  return null
}

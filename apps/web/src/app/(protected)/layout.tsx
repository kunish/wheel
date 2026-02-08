"use client"

import { useQueryClient } from "@tanstack/react-query"
import { useRouter } from "next/navigation"
import { useEffect, useState } from "react"
import { AppLayout } from "@/components/app-layout"
import { useStatsWebSocket } from "@/hooks/use-stats-ws"
import { useAuthStore } from "@/lib/store/auth"

export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const router = useRouter()
  const [checked, setChecked] = useState(false)
  const queryClient = useQueryClient()
  useStatsWebSocket(queryClient)

  useEffect(() => {
    if (!isAuthenticated()) {
      router.replace("/login")
    } else {
      setChecked(true)
    }
  }, [isAuthenticated, router])

  if (!checked) return null

  return <AppLayout>{children}</AppLayout>
}

"use client"

import { useQueryClient } from "@tanstack/react-query"
import { useRouter } from "next/navigation"
import { useEffect, useMemo } from "react"
import { AppLayout } from "@/components/app-layout"
import { useStatsWebSocket } from "@/hooks/use-stats-ws"
import { useAuthStore } from "@/lib/store/auth"

export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const router = useRouter()
  const queryClient = useQueryClient()
  useStatsWebSocket(queryClient)

  const checked = useMemo(() => isAuthenticated(), [isAuthenticated])

  useEffect(() => {
    if (!checked) {
      router.replace("/login")
    }
  }, [checked, router])

  if (!checked) return null

  return <AppLayout>{children}</AppLayout>
}

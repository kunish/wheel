"use client"

import { useQueryClient } from "@tanstack/react-query"
import { Loader2 } from "lucide-react"
import { useRouter } from "next/navigation"
import { useEffect } from "react"
import { AppLayout } from "@/components/app-layout"
import { useStatsWebSocket } from "@/hooks/use-stats-ws"
import { useAuthStore } from "@/lib/store/auth"

export default function ProtectedLayout({ children }: { children: React.ReactNode }) {
  const isAuthenticated = useAuthStore((s) => s.isAuthenticated)
  const router = useRouter()
  const queryClient = useQueryClient()
  useStatsWebSocket(queryClient)

  const ready = isAuthenticated()

  useEffect(() => {
    if (!ready) {
      router.replace("/login")
    }
  }, [ready, router])

  if (!ready)
    return (
      <div className="flex h-screen items-center justify-center">
        <Loader2 className="text-muted-foreground h-8 w-8 animate-spin" />
      </div>
    )

  return <AppLayout>{children}</AppLayout>
}

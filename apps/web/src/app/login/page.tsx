"use client"

import { Zap } from "lucide-react"
import { useRouter } from "next/navigation"
import { useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { login } from "@/lib/api"
import { useAuthStore } from "@/lib/store/auth"

export default function LoginPage() {
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const setAuth = useAuthStore((s) => s.setAuth)
  const router = useRouter()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setLoading(true)
    try {
      const res = await login(username, password)
      if (res.success) {
        setAuth(res.data.token, res.data.expireAt)
        router.push("/dashboard")
      }
    } catch {
      toast.error("Login failed. Please check your credentials.")
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-background flex min-h-screen items-center justify-center p-4">
      {/* Decorative background shapes */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="bg-nb-lime/20 border-border/10 absolute top-[10%] left-[5%] size-32 rotate-12 rounded-md border-2" />
        <div className="bg-nb-pink/20 border-border/10 absolute right-[8%] bottom-[15%] size-24 -rotate-6 rounded-md border-2" />
        <div className="bg-nb-sky/20 border-border/10 absolute top-[60%] left-[15%] size-16 rotate-45 rounded-md border-2" />
        <div className="bg-nb-orange/20 border-border/10 absolute top-[20%] right-[20%] size-20 -rotate-12 rounded-md border-2" />
      </div>

      <Card className="relative z-10 w-full max-w-sm shadow-[6px_6px_0_var(--nb-shadow)]">
        <CardHeader className="pb-2 text-center">
          <div className="mb-4 flex justify-center">
            <div className="bg-nb-lime border-border flex size-14 items-center justify-center rounded-lg border-2 shadow-[3px_3px_0_var(--nb-shadow)]">
              <Zap className="size-7 fill-current" />
            </div>
          </div>
          <CardTitle className="text-2xl">Wheel</CardTitle>
          <CardDescription>Sign in to manage your LLM gateway</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="username" className="text-xs font-bold tracking-wider uppercase">
                Username
              </Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus
                placeholder="Enter your username"
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password" className="text-xs font-bold tracking-wider uppercase">
                Password
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                placeholder="Enter your password"
              />
            </div>
            <Button type="submit" className="mt-2 w-full" disabled={loading}>
              {loading ? "Signing in..." : "Sign In"}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

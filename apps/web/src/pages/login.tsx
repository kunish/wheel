import { motion } from "motion/react"
import { useState } from "react"
import { useTranslation } from "react-i18next"
import { useNavigate } from "react-router"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { GearOutline, GearSpinner } from "@/components/ui/gear-spinner"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { login } from "@/lib/api-client"
import { useAuthStore } from "@/lib/store/auth"

export default function LoginPage() {
  const { t } = useTranslation("login")
  const [username, setUsername] = useState("")
  const [password, setPassword] = useState("")
  const [loading, setLoading] = useState(false)
  const [showApiUrl, setShowApiUrl] = useState(() => !!useAuthStore.getState().apiBaseUrl)
  const [apiUrl, setApiUrl] = useState(() => useAuthStore.getState().apiBaseUrl)
  const setAuth = useAuthStore((s) => s.setAuth)
  const setApiBaseUrl = useAuthStore((s) => s.setApiBaseUrl)
  const navigate = useNavigate()

  async function handleSubmit(e: React.FormEvent) {
    e.preventDefault()
    setApiBaseUrl(apiUrl)
    setLoading(true)
    try {
      const res = await login(username, password)
      if (res.success) {
        setAuth(res.data.token, res.data.expireAt)
        navigate("/dashboard")
      }
    } catch {
      toast.error(t("error.loginFailed"))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="bg-background flex min-h-screen items-center justify-center p-4">
      {/* Decorative gear background */}
      <div className="pointer-events-none fixed inset-0 overflow-hidden">
        <div className="animate-gear-spin-slow absolute top-[10%] left-[5%]">
          <GearOutline size={128} className="text-nb-lime/15" />
        </div>
        <div className="animate-gear-spin-reverse absolute right-[8%] bottom-[15%]">
          <GearOutline size={96} className="text-nb-pink/15" />
        </div>
        <div className="animate-gear-spin-slow absolute top-[60%] left-[15%]">
          <GearOutline size={64} className="text-nb-sky/15" />
        </div>
        <div className="animate-gear-spin-reverse absolute top-[20%] right-[20%]">
          <GearOutline size={80} className="text-nb-orange/15" />
        </div>
      </div>

      <Card className="relative z-10 w-full max-w-sm shadow-xl">
        <CardHeader className="pb-2 text-center">
          <motion.div
            className="mb-4 flex justify-center"
            initial={{ scale: 0, rotate: -180 }}
            animate={{ scale: 1, rotate: 0 }}
            transition={{ duration: 0.6, ease: "backOut" }}
          >
            <div className="bg-nb-lime flex size-14 items-center justify-center rounded-xl shadow-sm">
              <svg
                className="size-7"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
              >
                <circle cx="12" cy="12" r="9" />
                <circle cx="12" cy="12" r="3" />
                <line x1="12" y1="3" x2="12" y2="9" />
                <line x1="12" y1="15" x2="12" y2="21" />
                <line x1="3" y1="12" x2="9" y2="12" />
                <line x1="15" y1="12" x2="21" y2="12" />
                <line x1="5.64" y1="5.64" x2="9.88" y2="9.88" />
                <line x1="14.12" y1="14.12" x2="18.36" y2="18.36" />
                <line x1="18.36" y1="5.64" x2="14.12" y2="9.88" />
                <line x1="9.88" y1="14.12" x2="5.64" y2="18.36" />
              </svg>
            </div>
          </motion.div>
          <CardTitle className="text-2xl">{t("title")}</CardTitle>
          <CardDescription>{t("description")}</CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleSubmit} className="flex flex-col gap-4">
            {showApiUrl && (
              <div className="flex flex-col gap-2">
                <Label htmlFor="apiUrl" className="text-xs font-bold tracking-wider uppercase">
                  {t("apiUrl")}
                </Label>
                <Input
                  id="apiUrl"
                  type="url"
                  value={apiUrl}
                  onChange={(e) => setApiUrl(e.target.value)}
                  placeholder={t("apiUrlPlaceholder")}
                />
                <p className="text-muted-foreground text-xs">{t("apiUrlHint")}</p>
              </div>
            )}
            <div className="flex flex-col gap-2">
              <Label htmlFor="username" className="text-xs font-bold tracking-wider uppercase">
                {t("username")}
              </Label>
              <Input
                id="username"
                value={username}
                onChange={(e) => setUsername(e.target.value)}
                required
                autoFocus
                placeholder={t("usernamePlaceholder")}
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label htmlFor="password" className="text-xs font-bold tracking-wider uppercase">
                {t("password")}
              </Label>
              <Input
                id="password"
                type="password"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                placeholder={t("passwordPlaceholder")}
              />
            </div>
            <Button type="submit" className="mt-2 w-full" disabled={loading}>
              {loading ? (
                <>
                  <GearSpinner size="sm" speed="fast" className="mr-2" />
                  {t("submitting")}
                </>
              ) : (
                t("submit")
              )}
            </Button>
            {!showApiUrl && (
              <button
                type="button"
                className="text-muted-foreground hover:text-foreground text-xs underline transition-colors"
                onClick={() => setShowApiUrl(true)}
              >
                {t("configure")}
              </button>
            )}
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

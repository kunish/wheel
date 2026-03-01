import { useMutation } from "@tanstack/react-query"
import { lazy, Suspense, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { changePassword, changeUsername } from "@/lib/api-client"

// ───────────── Lazy-loaded sections ─────────────

const SystemConfigSection = lazy(() => import("./settings/system-config-section"))

const BackupSection = lazy(() => import("./settings/backup-section"))

const ConnectionSection = lazy(() => import("./settings/connection-section"))

const VersionSection = lazy(() => import("./settings/version-section"))

// ───────────── Account Section ─────────────

function AccountSection() {
  const { t } = useTranslation("settings")
  const [newUsername, setNewUsername] = useState("")
  const [newPassword, setNewPassword] = useState("")
  const [confirmPassword, setConfirmPassword] = useState("")

  const usernameMutation = useMutation({
    mutationFn: () => changeUsername(newUsername),
    onSuccess: () => {
      toast.success(t("account.usernameUpdated"))
      setNewUsername("")
    },
    onError: () => toast.error(t("account.usernameUpdateFailed")),
  })

  const passwordMutation = useMutation({
    mutationFn: () => changePassword(newPassword),
    onSuccess: () => {
      toast.success(t("account.passwordUpdated"))
      setNewPassword("")
      setConfirmPassword("")
    },
    onError: () => toast.error(t("account.passwordUpdateFailed")),
  })

  return (
    <div className="grid gap-6 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>{t("account.changeUsername")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (!newUsername.trim()) return
              usernameMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>{t("account.newUsername")}</Label>
              <Input
                value={newUsername}
                onChange={(e) => setNewUsername(e.target.value)}
                placeholder={t("account.enterNewUsername")}
                required
              />
            </div>
            <Button type="submit" disabled={usernameMutation.isPending}>
              {t("account.updateUsername")}
            </Button>
          </form>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t("account.changePassword")}</CardTitle>
        </CardHeader>
        <CardContent>
          <form
            onSubmit={(e) => {
              e.preventDefault()
              if (newPassword.length < 8) {
                toast.error(t("account.passwordMinLength"))
                return
              }
              if (newPassword !== confirmPassword) {
                toast.error(t("account.passwordsDoNotMatch"))
                return
              }
              if (!newPassword) return
              passwordMutation.mutate()
            }}
            className="flex flex-col gap-4"
          >
            <div className="flex flex-col gap-2">
              <Label>{t("account.newPassword")}</Label>
              <Input
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder={t("account.enterNewPassword")}
                required
              />
            </div>
            <div className="flex flex-col gap-2">
              <Label>{t("account.confirmPassword")}</Label>
              <Input
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder={t("account.confirmNewPassword")}
                required
              />
            </div>
            <Button type="submit" disabled={passwordMutation.isPending}>
              {t("account.updatePassword")}
            </Button>
          </form>
        </CardContent>
      </Card>
    </div>
  )
}

// ───────────── Suspense Fallback ─────────────

function SectionFallback() {
  const { t } = useTranslation("common")
  return <p className="text-muted-foreground">{t("actions.loading")}</p>
}

// ───────────── Settings Page ─────────────

export default function SettingsPage() {
  const { t } = useTranslation("settings")
  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <h2 className="shrink-0 pb-4 text-2xl font-bold tracking-tight">{t("title")}</h2>
      <Tabs defaultValue="account" className="flex min-h-0 flex-1 flex-col">
        <TabsList variant="line" className="shrink-0">
          <TabsTrigger value="account">{t("tabs.account")}</TabsTrigger>
          <TabsTrigger value="system">{t("tabs.system")}</TabsTrigger>
          <TabsTrigger value="connection">{t("tabs.connection")}</TabsTrigger>
          <TabsTrigger value="backup">{t("tabs.backup")}</TabsTrigger>
          <TabsTrigger value="version">{t("tabs.version")}</TabsTrigger>
        </TabsList>

        <TabsContent value="account" className="min-h-0 flex-1 overflow-auto pt-4">
          <AccountSection />
        </TabsContent>

        <TabsContent value="system" className="min-h-0 flex-1 overflow-auto pt-4">
          <Suspense fallback={<SectionFallback />}>
            <SystemConfigSection />
          </Suspense>
        </TabsContent>

        <TabsContent value="connection" className="min-h-0 flex-1 overflow-auto pt-4">
          <Suspense fallback={<SectionFallback />}>
            <ConnectionSection />
          </Suspense>
        </TabsContent>

        <TabsContent value="backup" className="min-h-0 flex-1 overflow-auto pt-4">
          <Suspense fallback={<SectionFallback />}>
            <BackupSection />
          </Suspense>
        </TabsContent>

        <TabsContent value="version" className="min-h-0 flex-1 overflow-auto pt-4">
          <Suspense fallback={<SectionFallback />}>
            <VersionSection />
          </Suspense>
        </TabsContent>
      </Tabs>
    </div>
  )
}

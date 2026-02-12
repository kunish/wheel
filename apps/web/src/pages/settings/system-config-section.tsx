import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useEffect, useMemo, useState } from "react"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getSettings, updateSettings } from "@/lib/api"

const SETTING_KEYS = [
  "log_retention_days",
  "circuit_breaker_threshold",
  "circuit_breaker_cooldown",
  "circuit_breaker_max_cooldown",
] as const

export default function SystemConfigSection() {
  const { t } = useTranslation("settings")
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ["settings"],
    queryFn: getSettings,
  })

  const settingLabels = useMemo(
    () =>
      Object.fromEntries(
        SETTING_KEYS.map((key) => [
          key,
          {
            label: t(`systemConfig.labels.${key}`),
            description: t(`systemConfig.descriptions.${key}`),
          },
        ]),
      ) as Record<string, { label: string; description: string }>,
    [t],
  )

  const [formData, setFormData] = useState<Record<string, string>>({})

  // Sync form data when settings load or change
  useEffect(() => {
    if (data?.data?.settings) {
      const filtered = Object.fromEntries(
        Object.entries(data.data.settings).filter(([key]) => key in settingLabels),
      )
      setFormData(filtered)
    }
  }, [data?.data?.settings, settingLabels])

  const mutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] })
      toast.success(t("systemConfig.settingsSaved"))
    },
  })

  if (isLoading) {
    return <p className="text-muted-foreground">{t("actions.loading", { ns: "common" })}</p>
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t("systemConfig.title")}</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground mb-4 text-sm">{t("systemConfig.description")}</p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            mutation.mutate(formData)
          }}
          className="flex flex-col gap-4"
        >
          {Object.entries(formData).map(([key, value]) => (
            <div key={key} className="flex flex-col gap-2">
              <Label>{settingLabels[key]?.label ?? key}</Label>
              {settingLabels[key]?.description && (
                <p className="text-muted-foreground text-xs">{settingLabels[key].description}</p>
              )}
              <Input
                type="number"
                min="0"
                value={value}
                onChange={(e) => setFormData((prev) => ({ ...prev, [key]: e.target.value }))}
              />
            </div>
          ))}
          {Object.keys(formData).length === 0 && (
            <p className="text-muted-foreground text-sm">{t("systemConfig.noSettings")}</p>
          )}
          <Button type="submit" disabled={mutation.isPending}>
            {t("systemConfig.saveChanges")}
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

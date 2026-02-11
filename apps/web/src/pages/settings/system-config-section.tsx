import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query"
import { useRef, useState } from "react"
import { toast } from "sonner"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { getSettings, updateSettings } from "@/lib/api"

const SETTING_LABELS: Record<string, { label: string; description?: string }> = {
  log_retention_days: {
    label: "Log Retention Days",
    description: "Days to keep relay logs (0 = unlimited)",
  },
  circuit_breaker_threshold: {
    label: "Circuit Breaker Threshold",
    description: "Consecutive failures before tripping (default: 5)",
  },
  circuit_breaker_cooldown: {
    label: "Circuit Breaker Cooldown (s)",
    description: "Base cooldown in seconds (default: 60)",
  },
  circuit_breaker_max_cooldown: {
    label: "Circuit Breaker Max Cooldown (s)",
    description: "Maximum cooldown in seconds after exponential backoff (default: 600)",
  },
}

export default function SystemConfigSection() {
  const queryClient = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ["settings"],
    queryFn: getSettings,
  })

  const [formData, setFormData] = useState<Record<string, string>>({})

  // Sync form data when settings load
  const prevSettingsRef = useRef(data?.data?.settings)
  if (prevSettingsRef.current !== data?.data?.settings) {
    prevSettingsRef.current = data?.data?.settings
    if (data?.data?.settings) {
      const filtered = Object.fromEntries(
        Object.entries(data.data.settings).filter(([key]) => key in SETTING_LABELS),
      )
      setFormData(filtered)
    }
  }

  const mutation = useMutation({
    mutationFn: updateSettings,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["settings"] })
      toast.success("Settings saved")
    },
  })

  if (isLoading) {
    return <p className="text-muted-foreground">Loading...</p>
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>System Configuration</CardTitle>
      </CardHeader>
      <CardContent>
        <p className="text-muted-foreground mb-4 text-sm">
          Configure system behavior. Changes take effect immediately.
        </p>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            mutation.mutate(formData)
          }}
          className="flex flex-col gap-4"
        >
          {Object.entries(formData).map(([key, value]) => (
            <div key={key} className="flex flex-col gap-2">
              <Label>{SETTING_LABELS[key]?.label ?? key}</Label>
              {SETTING_LABELS[key]?.description && (
                <p className="text-muted-foreground text-xs">{SETTING_LABELS[key].description}</p>
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
            <p className="text-muted-foreground text-sm">No settings configured yet.</p>
          )}
          <Button type="submit" disabled={mutation.isPending}>
            Save Changes
          </Button>
        </form>
      </CardContent>
    </Card>
  )
}

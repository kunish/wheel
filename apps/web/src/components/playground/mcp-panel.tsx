import type { UsePlaygroundMcpResult } from "@/hooks/use-playground-mcp"
import { Plug, Wrench } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Checkbox } from "@/components/ui/checkbox"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Switch } from "@/components/ui/switch"

interface McpPanelProps {
  mcp: UsePlaygroundMcpResult
  disabled?: boolean
}

export function McpPanel({ mcp, disabled }: McpPanelProps) {
  const { t } = useTranslation("playground")

  return (
    <Card>
      <CardHeader className="pb-3">
        <CardTitle className="flex items-center gap-2 text-base">
          <Plug className="h-4 w-4" />
          {t("mcp.title")}
          <Badge variant="outline" className="ml-auto text-[10px]">
            {t("mcp.selectedCount", { count: mcp.selectedCount })}
          </Badge>
        </CardTitle>
      </CardHeader>
      <CardContent className="space-y-3">
        <div className="flex items-center justify-between gap-2">
          <div className="space-y-0.5">
            <Label>{t("mcp.enable")}</Label>
            <p className="text-muted-foreground text-xs">{t("mcp.enableHint")}</p>
          </div>
          <Switch checked={mcp.enabled} onCheckedChange={mcp.setEnabled} disabled={disabled} />
        </div>

        <div className="space-y-2">
          <Label>{t("mcp.mode")}</Label>
          <Select
            value={mcp.mode}
            onValueChange={(value) => mcp.setMode(value as "auto" | "manual")}
            disabled={!mcp.enabled || disabled}
          >
            <SelectTrigger>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="auto">{t("mcp.modeAuto")}</SelectItem>
              <SelectItem value="manual">{t("mcp.modeManual")}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        <div className="flex items-center justify-between gap-2">
          <Label>{t("mcp.tools")}</Label>
          <div className="flex items-center gap-1">
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={mcp.selectAll}
              disabled={!mcp.enabled || disabled || mcp.tools.length === 0}
            >
              {t("mcp.selectAll")}
            </Button>
            <Button
              type="button"
              variant="ghost"
              size="sm"
              onClick={mcp.clearAll}
              disabled={!mcp.enabled || disabled || mcp.selectedKeys.length === 0}
            >
              {t("mcp.clearSelection")}
            </Button>
          </div>
        </div>

        <ScrollArea className="h-44 rounded-md border p-2">
          <div className="space-y-1">
            {mcp.tools.length === 0 ? (
              <p className="text-muted-foreground px-1 py-4 text-center text-xs">
                {t("mcp.noTools")}
              </p>
            ) : (
              mcp.tools.map((tool) => (
                <div
                  key={tool.key}
                  className="hover:bg-muted/40 flex items-start gap-2 rounded px-2 py-1.5"
                >
                  <Checkbox
                    checked={mcp.selectedKeys.includes(tool.key)}
                    onCheckedChange={() => mcp.toggleTool(tool.key)}
                    disabled={!mcp.enabled || disabled}
                  />
                  <div className="min-w-0 text-xs">
                    <div className="flex items-center gap-1.5">
                      <Wrench className="h-3 w-3 shrink-0" />
                      <span className="truncate font-mono">{tool.toolName}</span>
                      <Badge variant="outline" className="text-[10px]">
                        {tool.clientName}
                      </Badge>
                    </div>
                    {tool.description && (
                      <p className="text-muted-foreground mt-0.5 line-clamp-2">
                        {tool.description}
                      </p>
                    )}
                  </div>
                </div>
              ))
            )}
          </div>
        </ScrollArea>
      </CardContent>
    </Card>
  )
}

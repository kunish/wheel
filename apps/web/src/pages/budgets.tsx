import { useQuery } from "@tanstack/react-query"
import { AlertTriangle, ArrowRight, DollarSign, ShieldAlert, TrendingUp } from "lucide-react"
import { useTranslation } from "react-i18next"
import { Link } from "react-router"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { listApiKeys } from "@/lib/api-client"

function formatCost(cost: number) {
  return `$${cost.toFixed(4)}`
}

export default function BudgetsPage() {
  const { t } = useTranslation("budgets")

  const { data: keysData, isLoading } = useQuery({
    queryKey: ["api-keys"],
    queryFn: listApiKeys,
  })

  const keys = keysData?.data?.apiKeys ?? []
  const keysWithBudget = keys.filter((k) => k.maxCost > 0)
  const keysWithoutBudget = keys.filter((k) => k.maxCost <= 0)

  const totalBudget = keysWithBudget.reduce((sum, k) => sum + k.maxCost, 0)
  const totalSpent = keys.reduce((sum, k) => sum + k.totalCost, 0)
  const overBudgetCount = keysWithBudget.filter((k) => k.totalCost >= k.maxCost).length
  const nearLimitCount = keysWithBudget.filter(
    (k) => k.totalCost >= k.maxCost * 0.8 && k.totalCost < k.maxCost,
  ).length

  function getStatus(key: { totalCost: number; maxCost: number }) {
    if (key.maxCost <= 0) return "noLimit"
    if (key.totalCost >= key.maxCost) return "overBudget"
    if (key.totalCost >= key.maxCost * 0.8) return "nearLimit"
    return "healthy"
  }

  function getStatusVariant(status: string): "destructive" | "secondary" | "default" | "outline" {
    switch (status) {
      case "overBudget":
        return "destructive"
      case "nearLimit":
        return "secondary"
      case "noLimit":
        return "outline"
      default:
        return "default"
    }
  }

  const summaryCards = [
    {
      label: t("summary.totalBudget"),
      value: formatCost(totalBudget),
      icon: DollarSign,
    },
    {
      label: t("summary.totalSpent"),
      value: formatCost(totalSpent),
      icon: TrendingUp,
    },
    {
      label: t("summary.keysOverBudget"),
      value: String(overBudgetCount),
      icon: ShieldAlert,
    },
    {
      label: t("summary.keysNearLimit"),
      value: String(nearLimitCount),
      icon: AlertTriangle,
    },
  ]

  return (
    <div className="flex min-h-0 flex-1 flex-col">
      <div className="shrink-0 pb-4">
        <h2 className="text-2xl font-bold tracking-tight">{t("title")}</h2>
        <p className="text-muted-foreground text-sm">{t("description")}</p>
      </div>

      <div className="min-h-0 flex-1 space-y-6 overflow-auto">
        {/* Summary */}
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {summaryCards.map((card) => {
            const Icon = card.icon
            return (
              <Card key={card.label}>
                <CardContent className="flex items-center gap-4 pt-6">
                  <div className="bg-muted flex h-10 w-10 shrink-0 items-center justify-center rounded-lg">
                    <Icon className="h-5 w-5" />
                  </div>
                  <div>
                    <p className="text-muted-foreground text-xs">{card.label}</p>
                    <p className="text-xl font-bold">{card.value}</p>
                  </div>
                </CardContent>
              </Card>
            )
          })}
        </div>

        {/* Budget Table */}
        <Card>
          <CardHeader className="flex flex-row items-center justify-between">
            <CardTitle>{t("title")}</CardTitle>
            <Button variant="outline" size="sm" asChild>
              <Link to="/keys">
                {t("goToKeys")}
                <ArrowRight className="ml-2 h-4 w-4" />
              </Link>
            </Button>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <p className="text-muted-foreground py-8 text-center">
                {t("actions.loading", { ns: "common" })}
              </p>
            ) : keys.length === 0 ? (
              <div className="flex flex-col items-center justify-center gap-2 py-16">
                <DollarSign className="text-muted-foreground h-10 w-10" />
                <p className="text-muted-foreground font-medium">{t("noBudgets")}</p>
                <p className="text-muted-foreground text-sm">{t("noBudgetsHint")}</p>
              </div>
            ) : (
              <div className="overflow-x-auto">
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>{t("table.name")}</TableHead>
                      <TableHead className="text-right">{t("table.maxCost")}</TableHead>
                      <TableHead className="text-right">{t("table.totalCost")}</TableHead>
                      <TableHead className="text-right">{t("table.remaining")}</TableHead>
                      <TableHead>{t("table.percentage")}</TableHead>
                      <TableHead>{t("table.status")}</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {[...keysWithBudget, ...keysWithoutBudget]
                      .sort((a, b) => {
                        const sa = getStatus(a)
                        const sb = getStatus(b)
                        const order = { overBudget: 0, nearLimit: 1, healthy: 2, noLimit: 3 }
                        return (
                          (order[sa as keyof typeof order] ?? 4) -
                          (order[sb as keyof typeof order] ?? 4)
                        )
                      })
                      .map((key) => {
                        const status = getStatus(key)
                        const percent = key.maxCost > 0 ? (key.totalCost / key.maxCost) * 100 : 0
                        const remaining = key.maxCost > 0 ? key.maxCost - key.totalCost : 0

                        return (
                          <TableRow key={key.id}>
                            <TableCell className="font-medium">{key.name}</TableCell>
                            <TableCell className="text-right font-mono">
                              {key.maxCost > 0 ? formatCost(key.maxCost) : t("unlimited")}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {formatCost(key.totalCost)}
                            </TableCell>
                            <TableCell className="text-right font-mono">
                              {key.maxCost > 0 ? formatCost(Math.max(0, remaining)) : "—"}
                            </TableCell>
                            <TableCell className="w-40">
                              {key.maxCost > 0 ? (
                                <div className="flex items-center gap-2">
                                  <Progress value={Math.min(percent, 100)} className="h-2 w-24" />
                                  <span className="text-muted-foreground text-xs">
                                    {percent.toFixed(1)}%
                                  </span>
                                </div>
                              ) : (
                                <span className="text-muted-foreground text-xs">—</span>
                              )}
                            </TableCell>
                            <TableCell>
                              <Badge variant={getStatusVariant(status)}>{t(status)}</Badge>
                            </TableCell>
                          </TableRow>
                        )
                      })}
                  </TableBody>
                </Table>
              </div>
            )}
          </CardContent>
        </Card>
      </div>
    </div>
  )
}

import { AreaChart, BarChart3, LineChart } from "lucide-react"

export type ChartType = "area" | "line" | "bar"

interface ChartTypeToggleProps {
  value: ChartType
  onChange: (type: ChartType) => void
}

export function ChartTypeToggle({ value, onChange }: ChartTypeToggleProps) {
  const types: { type: ChartType; icon: typeof AreaChart; label: string }[] = [
    { type: "area", icon: AreaChart, label: "Area" },
    { type: "line", icon: LineChart, label: "Line" },
    { type: "bar", icon: BarChart3, label: "Bar" },
  ]

  return (
    <div className="bg-muted/50 flex items-center gap-0.5 rounded-md border p-0.5">
      {types.map(({ type, icon: Icon, label }) => (
        <button
          type="button"
          key={type}
          onClick={() => onChange(type)}
          className={`rounded-sm p-1 transition-colors ${
            value === type
              ? "bg-background text-foreground shadow-sm"
              : "text-muted-foreground hover:text-foreground"
          }`}
          title={label}
        >
          <Icon className="h-3.5 w-3.5" />
        </button>
      ))}
    </div>
  )
}

import type { DataPanelPopoverProps } from "./data-panel-popover"
import { DataPanelPopover } from "./data-panel-popover"

function NavArrow({
  direction,
  onClick,
  disabled,
}: {
  direction: "left" | "right"
  onClick: () => void
  disabled?: boolean
}) {
  return (
    <button
      onClick={onClick}
      disabled={disabled}
      className="text-muted-foreground hover:text-foreground hover:bg-muted hover:border-border rounded-md border-2 border-transparent p-1 transition-colors disabled:cursor-not-allowed disabled:opacity-30 disabled:hover:border-transparent disabled:hover:bg-transparent"
    >
      <svg width="14" height="14" viewBox="0 0 16 16" fill="none">
        <path
          d={direction === "left" ? "M10 4L6 8L10 12" : "M6 4L10 8L6 12"}
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </button>
  )
}

interface PeriodNavBarProps {
  label: string
  onPrev: () => void
  onNext: () => void
  nextDisabled?: boolean
  resetLabel?: string
  onReset?: () => void
  viewLogsLabel: string
  onViewLogs: () => void
  dataPanelProps: DataPanelPopoverProps
}

export function PeriodNavBar({
  label,
  onPrev,
  onNext,
  nextDisabled,
  resetLabel,
  onReset,
  viewLogsLabel,
  onViewLogs,
  dataPanelProps,
}: PeriodNavBarProps) {
  return (
    <div className="relative flex items-center gap-3">
      <NavArrow direction="left" onClick={onPrev} />
      <span className="text-base font-bold">{label}</span>
      <NavArrow direction="right" onClick={onNext} disabled={nextDisabled} />
      <DataPanelPopover {...dataPanelProps} />
      <div className="ml-auto flex items-center gap-3">
        {resetLabel && onReset && (
          <button
            onClick={onReset}
            className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
          >
            {resetLabel}
          </button>
        )}
        <button
          onClick={onViewLogs}
          className="text-muted-foreground hover:text-foreground text-xs font-medium underline-offset-2 hover:underline"
        >
          {viewLogsLabel}
        </button>
      </div>
    </div>
  )
}

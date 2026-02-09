"use client"

import { ChevronLeft, ChevronRight } from "lucide-react"
import * as React from "react"
import { DayPicker } from "react-day-picker"

import { buttonVariants } from "@/components/ui/button"
import { cn } from "@/lib/utils"

export type CalendarProps = React.ComponentProps<typeof DayPicker>

function ChevronIcon({ orientation }: { orientation?: string }) {
  const Icon = orientation === "left" ? ChevronLeft : ChevronRight
  return <Icon className="h-3.5 w-3.5" />
}

function Calendar({ className, classNames, showOutsideDays = true, ...props }: CalendarProps) {
  return (
    <DayPicker
      showOutsideDays={showOutsideDays}
      className={cn("p-3", className)}
      classNames={{
        months: "relative flex flex-col sm:flex-row gap-4",
        month: "flex flex-col gap-4",
        month_caption: "flex justify-center pt-1 items-center w-full",
        caption_label: "text-sm font-bold",
        nav: "absolute inset-x-0 top-0 flex items-center justify-between z-10 px-1 pt-[13px]",
        button_previous: cn(buttonVariants({ variant: "outline", size: "icon-xs" })),
        button_next: cn(buttonVariants({ variant: "outline", size: "icon-xs" })),
        month_grid: "w-full border-collapse",
        weekdays: "flex",
        weekday: "text-muted-foreground w-8 font-medium text-[0.75rem] text-center",
        week: "flex w-full mt-1",
        day: "relative p-0 text-center text-sm focus-within:relative",
        day_button: cn(
          buttonVariants({ variant: "ghost" }),
          "h-8 w-8 p-0 font-normal border-0 shadow-none hover:shadow-none active:shadow-none active:translate-x-0 active:translate-y-0 hover:translate-x-0 hover:translate-y-0",
        ),
        selected:
          "bg-primary text-primary-foreground hover:bg-primary hover:text-primary-foreground",
        today: "bg-accent text-accent-foreground font-bold",
        outside: "text-muted-foreground/40",
        disabled: "text-muted-foreground/30",
        range_middle:
          "bg-accent/40 text-accent-foreground rounded-none [&>button]:shadow-none [&>button]:border-0",
        range_start: "rounded-l-md",
        range_end: "rounded-r-md",
        hidden: "invisible",
        ...classNames,
      }}
      components={{
        Chevron: ChevronIcon,
      }}
      {...props}
    />
  )
}

export { Calendar }

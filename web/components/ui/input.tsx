import * as React from "react"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

function Input({ className, type, ...props }: React.ComponentProps<"input">) {
  return (
    <input
      type={type}
      data-slot="input"
      className={cn(
        // --border-strong (4.01:1 on a panel) is the WCAG 1.4.11 boundary for a
        // control. The --input token it replaces measured 1.44:1, i.e. a field
        // was a borderless rectangle. --elev-0 adds the 1px bevel so the field
        // reads as recessed rather than as a flat patch of a lighter navy.
        "h-8 w-full min-w-0 rounded-md border border-border-strong bg-surface-3 px-2.5 py-1 text-base text-fg-1 md:text-body-sm",
        "transition-[border-color,box-shadow,background-color] duration-(--dur-instant) ease-standard",
        "selection:bg-primary selection:text-primary-foreground placeholder:text-fg-3",
        "file:inline-flex file:h-7 file:border-0 file:bg-transparent file:text-body-sm file:font-medium file:text-fg-1",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        FOCUS_RING,
        "focus-visible:border-primary focus-visible:bg-surface-3 focus-visible:outline-0",
        "aria-invalid:border-destructive aria-invalid:focus-visible:outline-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Input }

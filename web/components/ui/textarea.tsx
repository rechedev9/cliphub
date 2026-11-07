import * as React from "react"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

/** A multi-line field with the same border, ground and focus as `Input`, so notes and names read as one form. */
function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        "min-h-20 w-full min-w-0 resize-y rounded-md border border-border-strong bg-surface-3 px-3.5 py-2.5 text-body-sm text-fg-1 shadow-[var(--elev-0)]",
        "transition-[border-color,box-shadow,background-color] duration-(--dur-instant) ease-standard",
        "selection:bg-primary selection:text-primary-foreground placeholder:text-fg-3",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        FOCUS_RING,
        "focus-visible:border-fg-3 focus-visible:bg-surface-4",
        "aria-invalid:border-destructive aria-invalid:focus-visible:outline-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }

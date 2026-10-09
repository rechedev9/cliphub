import * as React from "react"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

function Textarea({ className, ...props }: React.ComponentProps<"textarea">) {
  return (
    <textarea
      data-slot="textarea"
      className={cn(
        // Same field recipe as Input: --border-strong edge on --surface-3.
        "flex field-sizing-content min-h-20 w-full resize-y rounded-md border border-border-strong bg-surface-3 px-3.5 py-2 text-base text-fg-1 shadow-[var(--elev-0)] md:text-body",
        "transition-[border-color,box-shadow,background-color] duration-(--dur-instant) ease-standard",
        "selection:bg-primary selection:text-primary-foreground placeholder:text-fg-3",
        "disabled:pointer-events-none disabled:cursor-not-allowed disabled:opacity-50",
        FOCUS_RING,
        "focus-visible:border-primary focus-visible:bg-surface-4",
        "aria-invalid:border-destructive aria-invalid:focus-visible:outline-destructive",
        className
      )}
      {...props}
    />
  )
}

export { Textarea }

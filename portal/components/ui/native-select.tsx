import * as React from "react"
import { ChevronDownIcon } from "lucide-react"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

function NativeSelect({ className, ...props }: React.ComponentProps<"select">) {
  return (
    <div
      data-slot="native-select-wrapper"
      className="relative w-full has-[select:disabled]:opacity-50"
    >
      <select
        data-slot="native-select"
        className={cn(
          // Same field recipe as Input: --border-strong edge on --surface-3.
          "h-11 w-full min-w-0 appearance-none rounded-md border border-border-strong bg-surface-3 py-2 pr-9 pl-3.5 text-base text-fg-1 shadow-[var(--elev-0)] md:text-body",
          "transition-[border-color,box-shadow,background-color] duration-(--dur-instant) ease-standard",
          "disabled:pointer-events-none disabled:cursor-not-allowed",
          FOCUS_RING,
          "focus-visible:border-primary focus-visible:bg-surface-4",
          "aria-invalid:border-destructive aria-invalid:focus-visible:outline-destructive",
          className
        )}
        {...props}
      />
      <ChevronDownIcon
        aria-hidden="true"
        data-slot="native-select-icon"
        className="pointer-events-none absolute top-1/2 right-3 size-4 -translate-y-1/2 text-fg-3 select-none"
      />
    </div>
  )
}

function NativeSelectOption({ className, ...props }: React.ComponentProps<"option">) {
  return (
    <option
      data-slot="native-select-option"
      className={cn("bg-surface-3 text-fg-1", className)}
      {...props}
    />
  )
}

export { NativeSelect, NativeSelectOption }

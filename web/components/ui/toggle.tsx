"use client"

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Toggle as TogglePrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

const toggleVariants = cva(
  [
    "inline-flex items-center justify-center gap-2 rounded-md text-body-sm font-medium whitespace-nowrap",
    "transition-[color,background-color,border-color,box-shadow] duration-(--dur-fast) ease-standard",
    "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive",
    "data-[state=on]:border-primary data-[state=on]:bg-primary data-[state=on]:text-primary-foreground",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    FOCUS_RING,
  ],
  {
    variants: {
      variant: {
        default: "border border-transparent bg-transparent text-fg-2 hover:bg-surface-3 hover:text-fg-1",
        outline:
          "border border-border-strong bg-surface-3 text-fg-1 shadow-[var(--elev-0)] hover:border-primary/60 hover:bg-surface-4",
        // The Studio segmented control (design.md:110). This is the variant that
        // retires STUDIO_FILTER_CHIP_CLASS, a 400-character single-line string
        // pasted onto every filter chip in the app.
        filter:
          "border border-border-strong bg-surface-2 font-mono text-meta text-fg-2 uppercase hover:border-primary/60 hover:bg-primary/10 hover:text-fg-1 data-[state=on]:font-semibold data-[state=on]:shadow-[var(--elev-1),var(--glow-primary-sm)]",
      },
      size: {
        default: "h-10 min-w-10 px-3",
        sm: "h-9 min-w-9 px-2.5",
        lg: "h-11 min-w-11 px-3.5",
      },
    },
    compoundVariants: [
      // A filter chip is a 44px target with room for a word, not an icon box.
      { variant: "filter", size: "default", class: "h-11 min-w-16 px-4" },
    ],
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Toggle({
  className,
  variant,
  size,
  ...props
}: React.ComponentProps<typeof TogglePrimitive.Root> &
  VariantProps<typeof toggleVariants>) {
  return (
    <TogglePrimitive.Root
      data-slot="toggle"
      className={cn(toggleVariants({ variant, size, className }))}
      {...props}
    />
  )
}

export { Toggle, toggleVariants }

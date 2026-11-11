"use client"

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Toggle as TogglePrimitive } from "radix-ui"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

const toggleVariants = cva(
  [
    "inline-flex items-center justify-center gap-1.5 rounded-md text-body-sm font-medium whitespace-nowrap",
    "transition-[color,background-color,border-color,box-shadow] duration-(--dur-fast) ease-standard",
    "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive",
    "data-[state=on]:border-primary data-[state=on]:bg-primary/16 data-[state=on]:text-primary",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    FOCUS_RING,
  ],
  {
    variants: {
      variant: {
        default: "border border-transparent bg-transparent text-fg-2 hover:bg-surface-3 hover:text-fg-1",
        outline:
          "border border-border-strong bg-transparent text-fg-1 hover:border-fg-4 hover:bg-surface-3",
        // The Studio segmented control: every filter chip in the app.
        filter:
          "border border-border-strong bg-surface-2 text-meta text-fg-2 hover:border-fg-4 hover:bg-surface-3 hover:text-fg-1 data-[state=on]:font-semibold",
      },
      size: {
        default: "h-8 min-w-8 px-2.5",
        sm: "h-7 min-w-7 px-2",
        lg: "h-9 min-w-9 px-3",
      },
    },
    compoundVariants: [
      // A filter chip is a compact target with room for a word, not an icon box.
      { variant: "filter", size: "default", class: "h-8 min-w-14 px-3" },
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

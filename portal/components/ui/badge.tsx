import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"
import { FOCUS_RING } from "@/components/ui/button"

const badgeVariants = cva(
  [
    "inline-flex w-fit shrink-0 items-center justify-center gap-1 overflow-hidden border px-2 py-0.5 text-meta font-medium whitespace-nowrap",
    "transition-[color,background-color,border-color] duration-(--dur-instant) ease-standard",
    "[&>svg]:pointer-events-none [&>svg]:size-3",
    FOCUS_RING,
  ],
  {
    variants: {
      variant: {
        default: "border-transparent bg-primary text-primary-foreground [a&]:hover:bg-primary/90",
        secondary:
          "border-transparent bg-secondary text-secondary-foreground [a&]:hover:bg-surface-5",
        // White on --destructive is 3.17:1. --destructive-solid is 7.11:1.
        destructive:
          "border-transparent bg-destructive-solid text-white focus-visible:outline-destructive [a&]:hover:bg-destructive-solid/90",
        success: "border-success/40 bg-success/10 text-success",
        warning: "border-warning/40 bg-warning/10 text-warning",
        stream: "border-stream/40 bg-stream/10 text-stream-text",
        danger: "border-destructive/40 bg-destructive/10 text-destructive",
        outline: "border-border-strong text-fg-1 [a&]:hover:bg-surface-3",
        ghost: "border-transparent text-fg-2 [a&]:hover:bg-surface-3 [a&]:hover:text-fg-1",
        link: "border-transparent text-primary underline-offset-4 [a&]:hover:underline",
      },
      // The HUD language is square; `pill` stays the default so no existing
      // call site changes shape without asking for it.
      shape: {
        pill: "rounded-full",
        square: "rounded-none",
      },
    },
    defaultVariants: {
      variant: "default",
      shape: "pill",
    },
  }
)

function Badge({
  className,
  variant = "default",
  shape = "pill",
  asChild = false,
  ...props
}: React.ComponentProps<"span"> &
  VariantProps<typeof badgeVariants> & { asChild?: boolean }) {
  const Comp = asChild ? Slot.Root : "span"

  return (
    <Comp
      data-slot="badge"
      data-variant={variant}
      data-shape={shape}
      className={cn(badgeVariants({ variant, shape }), className)}
      {...props}
    />
  )
}

export { Badge, badgeVariants }

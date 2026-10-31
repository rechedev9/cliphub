"use client"

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Progress as ProgressPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

/**
 * The track is --surface-0 (a well), not `bg-primary/20`: a tinted track on a
 * panel measured 1.50:1 and the fill had to fight it. --surface-0 reads as a
 * recess on every panel step, so the fill is the only thing carrying colour.
 */
const progressVariants = cva(
  "relative w-full overflow-hidden rounded-full bg-surface-0 shadow-[var(--elev-0)]",
  {
    variants: {
      size: {
        xs: "h-1",
        sm: "h-1.5",
        md: "h-2",
      },
    },
    defaultVariants: {
      size: "md",
    },
  }
)

const progressIndicatorVariants = cva(
  "h-full w-full flex-1 transition-transform duration-(--dur-data) ease-standard",
  {
    variants: {
      tone: {
        primary: "bg-primary",
        stream: "bg-stream",
        success: "bg-success",
        warning: "bg-warning",
        destructive: "bg-destructive",
      },
    },
    defaultVariants: {
      tone: "primary",
    },
  }
)

/** Same three shell signals the Skeleton gate names; see skeleton.tsx. */
const INDETERMINATE_MOTION_GATE =
  "[html[data-performance-profile='efficiency']_&]:animate-none [html[data-window-activity='inactive']_&]:animate-none [html[data-capture-active='true']_&]:animate-none"

function Progress({
  className,
  value,
  size = "md",
  tone = "primary",
  indeterminate = false,
  ...props
}: React.ComponentProps<typeof ProgressPrimitive.Root> &
  VariantProps<typeof progressVariants> &
  VariantProps<typeof progressIndicatorVariants> & {
    /** Work is running but its extent is unknown. Never fake a percentage. */
    indeterminate?: boolean
  }) {
  return (
    <ProgressPrimitive.Root
      data-slot="progress"
      data-tone={tone}
      // Radix maps a null value to data-state="indeterminate" and drops
      // aria-valuenow, which is exactly the honest announcement here.
      value={indeterminate ? null : value}
      className={cn(progressVariants({ size }), className)}
      {...props}
    >
      <ProgressPrimitive.Indicator
        data-slot="progress-indicator"
        className={cn(
          progressIndicatorVariants({ tone }),
          indeterminate && `animate-pulse ${INDETERMINATE_MOTION_GATE}`
        )}
        style={
          indeterminate
            ? undefined
            : { transform: `translateX(-${100 - (value ?? 0)}%)` }
        }
      />
    </ProgressPrimitive.Root>
  )
}

export { Progress, progressVariants, progressIndicatorVariants }

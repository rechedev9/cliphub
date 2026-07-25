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

/** Literal glows cannot ride the `--glow-*` tokens' efficiency downgrade. */
const GLOW_EFFICIENCY_GATE =
  "[html[data-performance-profile='efficiency']_&]:shadow-none"

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
      glow: {
        true: "",
        false: "",
      },
    },
    compoundVariants: [
      // primary/stream have real glow tokens, so they inherit the efficiency
      // downgrade (the token resolves to `none`) for free.
      { tone: "primary", glow: true, class: "shadow-[var(--glow-primary-md)]" },
      { tone: "stream", glow: true, class: "shadow-[var(--glow-stream-md)]" },
      // The remaining tones have no glow token yet, so they are built from a
      // parseable literal plus a `shadow-<color>` — the one shape Tailwind can
      // splice a colour into. Writing the colour inside the arbitrary value
      // instead silently drops its alpha: `shadow-[0_0_18px_color-mix(…45%…)]`
      // compiles to `0 0 18px var(--tw-shadow-color, var(--success))`, i.e. a
      // fully opaque glow. Because the value is a literal it does not degrade
      // on its own, hence the explicit efficiency gate.
      {
        tone: "success",
        glow: true,
        class: `shadow-[0_0_18px] shadow-success/45 ${GLOW_EFFICIENCY_GATE}`,
      },
      {
        tone: "warning",
        glow: true,
        class: `shadow-[0_0_18px] shadow-warning/45 ${GLOW_EFFICIENCY_GATE}`,
      },
      {
        tone: "destructive",
        glow: true,
        class: `shadow-[0_0_18px] shadow-destructive/45 ${GLOW_EFFICIENCY_GATE}`,
      },
    ],
    defaultVariants: {
      tone: "primary",
      glow: false,
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
  glow = false,
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
          progressIndicatorVariants({ tone, glow }),
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

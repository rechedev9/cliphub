import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Loader2Icon } from "lucide-react"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

/**
 * design.md:80 — the single focus recipe every control in `components/ui` uses.
 *
 * It is an outline, not a ring, on purpose. `ring-offset-*` has to paint the
 * offset gap with a solid colour, so the recipe this replaces
 * (`ring-offset-background`) drew a canvas-coloured halo around any control
 * sitting on a panel, a popover or the sidebar — invisible while every surface
 * was the same black, obvious the moment the v4 ramp landed. An outline offset
 * is transparent, so one string is correct on every step of the ramp.
 */
export const FOCUS_RING =
  "focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-ring"

/**
 * Brand emphasis is expressed with `--glow-primary-*` / `--glow-stream-md`,
 * never with a `shadow-<color>` utility: Tailwind can only splice a shadow
 * colour into a value it can parse, and `--shadow-*` maps onto a bare
 * `var(--elev-N)`, so `shadow-primary/15` compiled to a variable that was
 * written and never read. The glow tokens also inherit the efficiency-profile
 * downgrade (they resolve to `none`), which a literal colour would not.
 */
const buttonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center gap-2 rounded-md text-body-sm font-semibold whitespace-nowrap",
    "transition-[background-color,border-color,box-shadow,color,opacity] duration-(--dur-fast) ease-standard",
    "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    FOCUS_RING,
  ],
  {
    variants: {
      variant: {
        default:
          "bg-primary text-primary-foreground shadow-[var(--elev-1),var(--glow-primary-sm)] hover:bg-primary/90 hover:shadow-[var(--elev-2),var(--glow-primary-md)] active:shadow-[var(--elev-0),var(--glow-primary-sm)] disabled:bg-secondary disabled:text-fg-3 disabled:shadow-[var(--elev-0)]",
        hero:
          "bg-primary font-display text-primary-foreground uppercase tracking-wide shadow-[var(--elev-2),var(--glow-primary-md)] hover:bg-primary/92 hover:shadow-[var(--elev-3),var(--glow-primary-lg)] active:shadow-[var(--elev-1),var(--glow-primary-md)] disabled:bg-secondary disabled:text-fg-3 disabled:shadow-[var(--elev-0)]",
        stream:
          "bg-stream text-stream-foreground shadow-[var(--elev-1),var(--glow-stream-md)] hover:bg-stream/90 hover:shadow-[var(--elev-2),var(--glow-stream-md)] active:shadow-[var(--elev-0)] focus-visible:outline-stream disabled:bg-secondary disabled:text-fg-3 disabled:shadow-[var(--elev-0)]",
        success:
          "bg-success text-success-foreground shadow-[var(--elev-1)] hover:bg-success/90 hover:shadow-[var(--elev-2)] active:shadow-[var(--elev-0)] disabled:bg-secondary disabled:text-fg-3",
        warning:
          "bg-warning text-warning-foreground shadow-[var(--elev-1)] hover:bg-warning/90 hover:shadow-[var(--elev-2)] active:shadow-[var(--elev-0)] disabled:bg-secondary disabled:text-fg-3",
        // --destructive under white text measures 3.17:1. --destructive-solid
        // is the only red in the palette that clears AA under white (7.11:1).
        destructive:
          "bg-destructive-solid text-white shadow-[var(--elev-1)] hover:bg-destructive-solid/90 hover:shadow-[var(--elev-2)] active:shadow-[var(--elev-0)] focus-visible:outline-destructive disabled:bg-secondary disabled:text-fg-3",
        outline:
          "border border-border-strong bg-surface-3 text-fg-1 shadow-[var(--elev-0)] hover:border-primary/60 hover:bg-surface-4 hover:text-fg-1 active:shadow-none disabled:bg-surface-2 disabled:text-fg-3",
        "outline-primary":
          "border border-primary/50 bg-surface-2 text-primary shadow-[var(--elev-0),var(--glow-primary-sm)] hover:border-primary hover:bg-primary/12 hover:shadow-[var(--elev-1),var(--glow-primary-md)] active:shadow-[var(--elev-0)] disabled:border-border-strong disabled:bg-surface-2 disabled:text-fg-3",
        secondary:
          "bg-secondary text-secondary-foreground shadow-[var(--elev-0)] hover:bg-surface-5 active:shadow-none",
        ghost: "text-fg-2 hover:bg-surface-3 hover:text-fg-1",
        link: "text-primary underline-offset-4 hover:underline",
      },
      size: {
        default: "h-11 px-5 py-2.5 has-[>svg]:px-4",
        xs: "h-8 gap-1 px-2.5 text-meta has-[>svg]:px-2 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-10 gap-1.5 px-3.5 has-[>svg]:px-3",
        lg: "h-12 px-7 text-body has-[>svg]:px-5",
        icon: "size-11",
        "icon-xs": "size-8 [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-10",
        "icon-lg": "size-12",
      },
    },
    defaultVariants: {
      variant: "default",
      size: "default",
    },
  }
)

function Button({
  className,
  variant = "default",
  size = "default",
  asChild = false,
  loading = false,
  disabled,
  children,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
    loading?: boolean
  }) {
  const Comp = asChild ? Slot.Root : "button"

  // Slot forwards to exactly one child, so the spinner slot and the implicit
  // `disabled` only exist on a real <button>; an `asChild` caller (a Link, an
  // <a>) owns its own busy affordance.
  const busy = loading && !asChild

  return (
    <Comp
      data-slot="button"
      data-variant={variant}
      data-size={size}
      data-loading={busy ? "true" : undefined}
      aria-busy={busy ? true : undefined}
      disabled={asChild ? disabled : disabled === true || busy}
      className={cn(buttonVariants({ variant, size, className }))}
      {...props}
    >
      {busy ? (
        <>
          <Loader2Icon aria-hidden className="animate-spin" />
          {children}
        </>
      ) : (
        children
      )}
    </Comp>
  )
}

export { Button, buttonVariants }

import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"
import { Loader2Icon } from "lucide-react"
import { Slot } from "radix-ui"

import { cn } from "@/lib/utils"

/**
 * The shared focus recipe for controls in `components/ui`.
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
 * EDGE controls: rectangular, 1px border, no shadow. `primary` is the one
 * accent, neon green: the main action is a green outline on a dark green tint
 * with caps text, the secondary is a grey outline and the destructive one a
 * red outline. Other colours keep their meaning (sides, outcomes, formats,
 * stream, errors).
 */
const CAPS = "uppercase tracking-wider"

const buttonVariants = cva(
  [
    "inline-flex shrink-0 items-center justify-center gap-1.5 rounded-md border border-transparent text-body-sm font-semibold whitespace-nowrap",
    "transition-[background-color,border-color,color,opacity] duration-(--dur-fast) ease-standard",
    "disabled:pointer-events-none disabled:opacity-50 aria-invalid:border-destructive",
    "[&_svg]:pointer-events-none [&_svg]:shrink-0 [&_svg:not([class*='size-'])]:size-4",
    FOCUS_RING,
  ],
  {
    variants: {
      variant: {
        default: `${CAPS} border-primary bg-primary/14 text-primary hover:bg-primary/24 disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        hero: `${CAPS} border-primary bg-primary/14 text-primary hover:bg-primary/24 disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        stream: `${CAPS} border-stream bg-stream/14 text-stream-text hover:bg-stream/24 focus-visible:outline-stream disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        success: `${CAPS} border-success bg-success/14 text-success hover:bg-success/24 disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        warning: `${CAPS} border-warning bg-warning/14 text-warning hover:bg-warning/24 disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        destructive: `${CAPS} border-destructive bg-destructive/12 text-destructive hover:bg-destructive/22 focus-visible:outline-destructive disabled:border-border-strong disabled:bg-transparent disabled:text-fg-3`,
        outline: `${CAPS} border-border-strong bg-transparent text-fg-1 hover:border-fg-4 hover:bg-surface-3 disabled:text-fg-3`,
        "outline-primary": `${CAPS} border-border-strong bg-transparent text-fg-1 hover:border-fg-3 hover:bg-surface-3 disabled:text-fg-3`,
        secondary: `${CAPS} border-border-strong bg-surface-3 text-secondary-foreground hover:border-fg-4 hover:bg-surface-5`,
        ghost: "text-fg-2 hover:bg-surface-3 hover:text-fg-1",
        link: "text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1",
      },
      size: {
        default: "h-8 px-3.5 has-[>svg]:px-3",
        xs: "h-6 gap-1 px-2 text-meta has-[>svg]:px-1.5 [&_svg:not([class*='size-'])]:size-3",
        sm: "h-7 gap-1.5 px-3 has-[>svg]:px-2.5",
        lg: "h-9 px-5 text-body has-[>svg]:px-4",
        icon: "size-8",
        "icon-xs": "size-6 [&_svg:not([class*='size-'])]:size-3",
        "icon-sm": "size-7",
        "icon-lg": "size-9",
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
  loadingText,
  disabled,
  children,
  ...props
}: React.ComponentProps<"button"> &
  VariantProps<typeof buttonVariants> & {
    asChild?: boolean
    loading?: boolean
    loadingText?: string
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
          {loadingText !== undefined ? loadingText : children}
        </>
      ) : (
        children
      )}
    </Comp>
  )
}

export { Button, buttonVariants }

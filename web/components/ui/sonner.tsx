"use client"

import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useTheme } from "next-themes"
import { Toaster as Sonner, type ToasterProps } from "sonner"

/**
 * sonner's own injected stylesheet hardcodes a system sans stack and reads its
 * colours from `--normal-*` custom properties on the toaster root, so the skin
 * has to be delivered as inline style on that element — an inline declaration
 * always wins over a stylesheet rule regardless of selector specificity.
 * Typing the custom properties here is what keeps this off the `as
 * React.CSSProperties` escape hatch.
 */
type ToasterStyle = React.CSSProperties & Record<`--${string}`, string>

const TOASTER_STYLE: ToasterStyle = {
  "--normal-bg": "var(--popover)",
  "--normal-text": "var(--popover-foreground)",
  "--normal-border": "var(--border-strong)",
  "--border-radius": "var(--radius)",
  fontFamily: "var(--font-sans)",
}

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()

  return (
    <Sonner
      theme={theme as ToasterProps["theme"]}
      className="toaster group"
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={TOASTER_STYLE}
      toastOptions={{
        // Per-type accents layer on top of the shared night-navy skin by
        // overriding the --normal-* custom properties the injected stylesheet
        // already reads border/text colour from, instead of adding Tailwind
        // colour utilities directly (those would lose to sonner's own
        // higher-specificity, unlayered `border`/`color` declarations).
        // A toast floats highest in the shell, so it carries --elev-5.
        classNames: {
          toast: "shadow-[var(--elev-5)]",
          success:
            "[--normal-border:var(--success)] [--normal-text:var(--success)]",
          warning:
            "[--normal-border:var(--warning)] [--normal-text:var(--warning)]",
          info: "[--normal-border:var(--primary)] [--normal-text:var(--primary)]",
          error:
            "[--normal-border:var(--destructive)] [--normal-text:var(--destructive)]",
        },
      }}
      {...props}
    />
  )
}

export { Toaster }

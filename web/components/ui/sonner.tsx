"use client"

import { useEffect } from "react"
import {
  CircleCheckIcon,
  InfoIcon,
  Loader2Icon,
  OctagonXIcon,
  TriangleAlertIcon,
} from "lucide-react"
import { useTheme } from "next-themes"
import { toast, Toaster as Sonner, type ToasterProps } from "sonner"

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

const TOP_OFFSET = { top: "calc(var(--shell-strip-height, 3.5rem) + 0.5rem)" }

/** Sonner only collapses an expanded list on Escape, and only when that list is focused. */
function useDismissToastsOnEscape(): void {
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent): void {
      if (event.key !== "Escape" || event.defaultPrevented) return
      toast.dismiss()
    }
    document.addEventListener("keydown", onKeyDown)
    return () => document.removeEventListener("keydown", onKeyDown)
  }, [])
}

const Toaster = ({ ...props }: ToasterProps) => {
  const { theme = "system" } = useTheme()
  useDismissToastsOnEscape()

  return (
    <Sonner
      {...props}
      theme={theme as ToasterProps["theme"]}
      icons={{
        success: <CircleCheckIcon className="size-4" />,
        info: <InfoIcon className="size-4" />,
        warning: <TriangleAlertIcon className="size-4" />,
        error: <OctagonXIcon className="size-4" />,
        loading: <Loader2Icon className="size-4 animate-spin" />,
      }}
      style={TOASTER_STYLE}
      // Top toasts open under the command strip instead of covering it.
      offset={TOP_OFFSET}
      mobileOffset={TOP_OFFSET}
      toastOptions={{
        closeButtonAriaLabel: "Cerrar aviso",
        // Per-type accents layer on top of the shared night-navy skin by
        // overriding the --normal-* custom properties the injected stylesheet
        // already reads border/text colour from, instead of adding Tailwind
        // colour utilities directly (those would lose to sonner's own
        // higher-specificity, unlayered `border`/`color` declarations).
        // A toast floats highest in the shell, so it carries --elev-5.
        classNames: {
          toast: "pointer-events-auto shadow-[var(--elev-5)]",
          closeButton: "pointer-events-auto",
          description: "text-fg-2!",
          actionButton: "bg-primary! text-primary-foreground! font-mono uppercase tracking-wide focus-visible:outline-2 focus-visible:outline-ring",
          success:
            "[--normal-border:var(--success)] [--normal-text:var(--success)]",
          warning:
            "[--normal-border:var(--warning)] [--normal-text:var(--warning)]",
          info: "[--normal-border:var(--primary)] [--normal-text:var(--primary)]",
          error:
            "[--normal-border:var(--destructive)] [--normal-text:var(--destructive)]",
        },
      }}
      // Clear of the sticky «Crear vídeo largo» bar. The offset above clears the command strip.
      position="top-right"
      closeButton
      // The list is wider than the card (full width under 600px). Clicks on
      // the empty list fall through; the card and its close button stay live.
      className="toaster group pointer-events-none"
    />
  )
}

export { Toaster }

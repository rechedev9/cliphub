import * as React from "react"
import { cva, type VariantProps } from "class-variance-authority"

import { cn } from "@/lib/utils"

/**
 * `.studio-panel` already paints --surface-2 + bevel + --elev-1 and owns the
 * radius. The old `rounded-xl` override made a Card a different shape (14px)
 * from every bare `studio-panel` (10px) in the same view, so it is gone; the
 * elevation is a prop instead of a shadow utility bolted on at the call site.
 */
const cardVariants = cva(
  "studio-panel flex flex-col gap-6 py-6 text-card-foreground",
  {
    variants: {
      elevation: {
        0: "shadow-[var(--elev-0)]",
        1: "shadow-[var(--elev-1)]",
        2: "shadow-[var(--elev-2)]",
        3: "shadow-[var(--elev-3)]",
      },
      tone: {
        neutral: "",
        accent: "border-border-accent",
        danger: "border-destructive/45",
        stream: "border-stream/45",
      },
    },
    defaultVariants: {
      elevation: 1,
      tone: "neutral",
    },
  }
)

function Card({
  className,
  elevation = 1,
  tone = "neutral",
  ...props
}: React.ComponentProps<"div"> & VariantProps<typeof cardVariants>) {
  return (
    <div
      data-slot="card"
      data-tone={tone}
      data-elevation={elevation}
      className={cn(cardVariants({ elevation, tone }), className)}
      {...props}
    />
  )
}

function CardHeader({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-header"
      className={cn(
        "@container/card-header grid auto-rows-min grid-rows-[auto_auto] items-start gap-2 px-6 has-data-[slot=card-action]:grid-cols-[1fr_auto] [.border-b]:pb-6",
        className
      )}
      {...props}
    />
  )
}

function CardTitle({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-title"
      className={cn(
        "font-display text-title leading-none font-bold text-fg-1",
        className
      )}
      {...props}
    />
  )
}

function CardDescription({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-description"
      className={cn("text-body-sm text-fg-2", className)}
      {...props}
    />
  )
}

function CardAction({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-action"
      className={cn(
        "col-start-2 row-span-2 row-start-1 self-start justify-self-end",
        className
      )}
      {...props}
    />
  )
}

function CardContent({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-content"
      className={cn("px-6", className)}
      {...props}
    />
  )
}

function CardFooter({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="card-footer"
      className={cn("flex items-center px-6 [.border-t]:pt-6", className)}
      {...props}
    />
  )
}

export {
  Card,
  CardHeader,
  CardFooter,
  CardTitle,
  CardAction,
  CardDescription,
  CardContent,
  cardVariants,
}

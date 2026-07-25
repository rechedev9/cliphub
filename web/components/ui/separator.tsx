"use client"

import * as React from "react"
import { Separator as SeparatorPrimitive } from "radix-ui"

import { cn } from "@/lib/utils"

function Separator({
  className,
  orientation = "horizontal",
  decorative = true,
  tone = "default",
  ...props
}: React.ComponentProps<typeof SeparatorPrimitive.Root> & {
  /** `subtle` is the divider *inside* a panel; `default` separates panels. */
  tone?: "default" | "subtle"
}) {
  return (
    <SeparatorPrimitive.Root
      data-slot="separator"
      data-tone={tone}
      decorative={decorative}
      orientation={orientation}
      className={cn(
        "shrink-0 data-[orientation=horizontal]:h-px data-[orientation=horizontal]:w-full data-[orientation=vertical]:h-full data-[orientation=vertical]:w-px",
        tone === "subtle" ? "bg-border-subtle" : "bg-border",
        className
      )}
      {...props}
    />
  )
}

export { Separator }

import type * as React from "react"

import { cn } from "@/lib/utils"

function LoadingText({ className, children, spinner = false, ...props }: React.ComponentProps<"p"> & { spinner?: boolean }) {
  return (
    <p
      role="status"
      aria-live="polite"
      data-slot="loading-text"
      className={cn("flex items-center gap-2 text-body-sm text-fg-2", className)}
      {...props}
    >
      {spinner ? <span aria-hidden className="studio-spinner" /> : null}
      {children}
    </p>
  )
}

export { LoadingText }

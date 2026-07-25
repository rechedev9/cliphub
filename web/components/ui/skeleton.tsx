import { cn } from "@/lib/utils"

/**
 * The depth gate (`--shell-depth`) cannot stop an animation, so the three shell
 * signals that must silence the compositor are named here as variants. The
 * fourth signal, `prefers-reduced-motion`, is already handled globally in
 * globals.css, which clamps every animation to a single 1ms iteration.
 */
const SKELETON_MOTION_GATE =
  "[html[data-performance-profile='efficiency']_&]:animate-none [html[data-window-activity='inactive']_&]:animate-none [html[data-capture-active='true']_&]:animate-none"

function Skeleton({ className, ...props }: React.ComponentProps<"div">) {
  return (
    <div
      data-slot="skeleton"
      className={cn(
        // --skeleton measures 1.89:1 on a panel; the bg-accent it replaces was
        // 1.23:1, i.e. less visible than the panel border it sat inside.
        "animate-pulse rounded-md bg-skeleton shadow-[var(--elev-0)]",
        // Tied to a motion token so reduced-motion collapses it through the
        // same scale as everything else instead of an invented duration.
        "[animation-duration:calc(var(--dur-data)*2.5)] [animation-timing-function:var(--ease-standard)]",
        SKELETON_MOTION_GATE,
        className
      )}
      {...props}
    />
  )
}

export { Skeleton }

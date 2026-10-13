import { cn } from "@/lib/utils";

export type StatusTone = "ok" | "live" | "warn" | "bad" | "off";

const DOT: Record<StatusTone, string> = {
  ok: "bg-success shadow-[0_0_0_3px_color-mix(in_oklch,var(--success)_20%,transparent)]",
  live: "bg-stream shadow-[0_0_0_3px_color-mix(in_oklch,var(--stream)_22%,transparent)]",
  warn: "bg-warning",
  bad: "bg-destructive",
  off: "bg-fg-4",
};

export const TONE_TEXT: Record<StatusTone, string> = {
  ok: "text-success",
  live: "text-stream-text",
  warn: "text-warning",
  bad: "text-destructive",
  off: "text-fg-3",
};

export function StatusDot({ tone, className }: { tone: StatusTone; className?: string }) {
  return <span aria-hidden="true" className={cn("size-2.5 shrink-0 rounded-full", DOT[tone], className)} />;
}

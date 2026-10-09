import { cn } from "@/lib/utils";

// Magenta is the product's recording colour: a live-capture cue, not decoration.
export function RecDot({ className }: { className?: string }) {
  return (
    <span
      aria-hidden="true"
      className={cn(
        "size-2 shrink-0 rounded-full bg-stream shadow-[0_0_0_3px_color-mix(in_oklch,var(--stream)_18%,transparent)]",
        className,
      )}
    />
  );
}

export function Eyebrow({ children }: { children: React.ReactNode }) {
  return (
    <p className="mb-5 inline-flex items-center gap-2 font-mono text-meta tracking-[0.18em] text-fg-3 uppercase">
      <RecDot />
      {children}
    </p>
  );
}

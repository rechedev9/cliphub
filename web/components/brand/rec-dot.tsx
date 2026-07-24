import { cn } from '@/lib/utils';

export type RecDotProps = {
  /** Optional caption next to the dot; defaults to "LIVE ON YOUR RIG". */
  label?: string;
  /** Render the dot only, no label. */
  hideLabel?: boolean;
  className?: string;
};

/**
 * RecDot — a small pulsing magenta dot with an optional "LIVE ON YOUR RIG"
 * label, shown on videos currently capturing on the player's machine. Magenta
 * is reserved for this REC indicator and other stream-specific signals; the
 * caption uses --stream-text, the ramp step measured for text, rather than the
 * fill colour. The bloom rides --glow-stream-md, so the efficiency profile
 * nulls it through the foundation. Honors reduced motion.
 */
export function RecDot({ label = 'LIVE ON YOUR RIG', hideLabel = false, className }: RecDotProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <span className="relative grid size-2.5 place-items-center">
        <span className="neon-pulse absolute inline-flex size-2.5 rounded-full bg-stream/40" />
        <span className="relative inline-flex size-1.5 rounded-full bg-stream [box-shadow:var(--glow-stream-md)] forced-colors:[box-shadow:none]" />
      </span>
      {!hideLabel ? (
        <span className="font-mono text-meta uppercase text-stream-text">{label}</span>
      ) : null}
    </span>
  );
}

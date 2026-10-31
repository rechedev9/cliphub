import { cn } from '@/lib/utils';

export type RecDotProps = {
  /** Optional caption next to the dot; defaults to "Live on your rig". */
  label?: string;
  /** Render the dot only, no label. */
  hideLabel?: boolean;
  className?: string;
};

/**
 * RecDot — a small pulsing magenta dot with an optional "Live on your rig"
 * label, shown on videos currently capturing on the player's machine. Magenta
 * is reserved for this REC indicator and other stream-specific signals; the
 * caption uses --stream-text, the ramp step measured for text, rather than the
 * fill colour. Honors reduced motion.
 */
export function RecDot({ label = 'Live on your rig', hideLabel = false, className }: RecDotProps) {
  return (
    <span className={cn('inline-flex items-center gap-2', className)}>
      <span className="relative grid size-2.5 place-items-center">
        <span className="live-pulse absolute inline-flex size-2.5 rounded-full bg-stream/40" />
        <span className="relative inline-flex size-1.5 rounded-full bg-stream forced-colors:[box-shadow:none]" />
      </span>
      {!hideLabel ? (
        <span className="text-meta text-stream-text">{label}</span>
      ) : null}
    </span>
  );
}

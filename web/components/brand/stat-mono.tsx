import { cn } from '@/lib/utils';

export type StatMonoProps = {
  /** Short uppercase caption, e.g. "K", "D", "K/D". */
  label: string;
  /** The number/score; rendered mono with tabular figures. */
  value: string | number;
  /** Stack label above value (default) or place it inline before the value. */
  layout?: 'stacked' | 'inline';
  /** Tint the value with the cyan signal color (e.g. a standout stat). */
  accent?: boolean;
  className?: string;
};

/** The label is the app's smallest legible step: 12px, --fg-3 (6.26:1 on a
 *  panel), and the widest tracking the scale allows, so it frames the number
 *  without competing with it. */
const LABEL_CLASS = 'font-mono text-meta uppercase tracking-widest text-fg-3';

/**
 * StatMono — a labeled mono number, NEON HUD style. Every stat in ClipHub
 * (K / D / A / MVP / K/D / scores / ticks / durations) is rendered through
 * this: a Share Tech Mono tabular value at the scoreboard step over a dim
 * wide-tracked label, so the scoreboard/demo-tick feel is consistent.
 *
 * The size step lives on the wrapper and the value inherits it, so a caller
 * that needs a tighter strip can drop one type utility into `className`
 * instead of re-typing the whole treatment.
 */
export function StatMono({
  label,
  value,
  layout = 'stacked',
  accent = false,
  className,
}: StatMonoProps) {
  const valueClass = cn('font-mono tabular-nums', accent ? 'text-primary' : 'text-fg-1');

  if (layout === 'inline') {
    return (
      <span className={cn('inline-flex items-baseline gap-1.5 text-body-lg', className)}>
        <span className={LABEL_CLASS}>{label}</span>
        <span className={valueClass}>{value}</span>
      </span>
    );
  }

  return (
    <div className={cn('flex min-w-0 flex-col gap-1 text-stat', className)}>
      <span className={LABEL_CLASS}>{label}</span>
      <span className={valueClass}>{value}</span>
    </div>
  );
}

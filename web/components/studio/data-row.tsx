import type { LucideIcon } from 'lucide-react';
import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type StudioDataRowProps = {
  /** The row's identity — file name, map, setting name. */
  label: ReactNode;
  /** Operational value, typeset as mono with tabular figures. */
  value?: ReactNode;
  /** Right-most state slot: a `StatusTag`, a spinner, a check. */
  status?: ReactNode;
  icon?: LucideIcon;
  /** Raise the row while it is the current subject of an operation. */
  active?: boolean;
  className?: string;
};

/**
 * Label / value / status row — the upload scan and parse lists, series parts and
 * the settings readouts, which shipped as four byte-identical hand-rolled class
 * strings. 44px minimum so a row lines up with the control scale even though it
 * is not itself a control.
 */
export function StudioDataRow({
  label,
  value,
  status,
  icon: Icon,
  active = false,
  className,
}: StudioDataRowProps): ReactNode {
  return (
    <div
      className={cn(
        'flex min-h-11 items-center justify-between gap-3 border px-3.5 py-2.5',
        'transition-colors duration-(--dur-fast) ease-standard',
        active ? 'border-border-accent bg-surface-3' : 'border-border bg-surface-2',
        className,
      )}
    >
      <span className="flex min-w-0 items-center gap-2 font-display text-body-sm font-bold uppercase tracking-wide text-fg-1">
        {Icon ? <Icon aria-hidden className="size-4 shrink-0 text-fg-3" /> : null}
        <span className="truncate">{label}</span>
      </span>
      {value !== undefined || status !== undefined ? (
        <span className="flex shrink-0 items-center gap-2.5">
          {value !== undefined ? <span className="font-mono text-body-sm tabular-nums text-fg-2">{value}</span> : null}
          {status}
        </span>
      ) : null}
    </div>
  );
}

import type { ReactNode } from 'react';
import { cn } from '@/lib/utils';

export type EdgeTone = 'good' | 'bad' | 'ct' | 't' | 'neutral';

const EDGE: Record<EdgeTone, string> = {
  good: 'border-l-success',
  bad: 'border-l-destructive',
  ct: 'border-l-side-ct',
  t: 'border-l-side-t',
  neutral: 'border-l-border-strong',
};

/** The EDGE tile: a 2px edge in the tone, a tiny caps label, a big number and an optional line under it. */
export function EdgeTile({
  label,
  value,
  note,
  detail,
  info,
  tone = 'neutral',
  wrapLabel = false,
  className,
}: {
  label: string;
  /** Lets a long label take a second line, as EDGE's overall tiles do, instead of cutting it. */
  wrapLabel?: boolean;
  value: ReactNode;
  note?: ReactNode;
  /** A second dim line under the note, such as the group mean. */
  detail?: ReactNode;
  /** Sits right of the label, such as the info tip that explains the formula. */
  info?: ReactNode;
  tone?: EdgeTone;
  className?: string;
}): ReactNode {
  return (
    <div className={cn('flex min-w-0 flex-col gap-0.5 border border-l-2 border-border bg-surface-2 px-3 py-2', EDGE[tone], className)}>
      <dt className={cn('flex min-w-0 gap-1 text-meta font-medium tracking-wider text-fg-3 uppercase', wrapLabel ? 'items-start' : 'items-center')}>
        <span className={wrapLabel ? 'min-w-0' : 'truncate'}>{label}</span>
        {info}
      </dt>
      <dd className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate font-num text-stat leading-tight font-bold text-fg-1 tabular-nums">{value}</span>
        {note ? <span className="truncate text-meta text-fg-3">{note}</span> : null}
        {detail ? <span className="truncate text-meta text-fg-3">{detail}</span> : null}
      </dd>
    </div>
  );
}

/** Tiles in a grid, as many per row as fit. */
export function EdgeTileGrid({ label, children, className }: { label: string; children: ReactNode; className?: string }): ReactNode {
  return (
    <dl aria-label={label} className={cn('grid grid-cols-[repeat(auto-fit,minmax(9rem,1fr))] gap-2', className)}>
      {children}
    </dl>
  );
}

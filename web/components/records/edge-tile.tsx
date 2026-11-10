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
  tone = 'neutral',
  className,
}: {
  label: string;
  value: ReactNode;
  note?: ReactNode;
  tone?: EdgeTone;
  className?: string;
}): ReactNode {
  return (
    <div className={cn('flex min-w-0 flex-col gap-0.5 border border-l-2 border-border bg-surface-2 px-3 py-2', EDGE[tone], className)}>
      <dt className="truncate text-meta font-medium tracking-wider text-fg-3 uppercase">{label}</dt>
      <dd className="flex min-w-0 flex-col gap-0.5">
        <span className="truncate text-stat leading-tight font-bold text-fg-1 tabular-nums">{value}</span>
        {note ? <span className="truncate text-meta text-fg-3">{note}</span> : null}
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

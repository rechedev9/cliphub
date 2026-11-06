'use client';

import type { ComponentProps, ReactNode } from 'react';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** A small toggle of the replay's dense panels: pressed reads as the accent edge, not a fill. */
export function ReplayChip({ pressed, className, children, ...props }: ComponentProps<'button'> & { pressed: boolean }): ReactNode {
  return (
    <button
      type="button"
      aria-pressed={pressed}
      className={cn(
        'inline-flex h-7 items-center gap-1.5 rounded-sm border px-2 text-meta whitespace-nowrap outline-none',
        'transition-[background-color,border-color,color] duration-(--dur-fast) motion-safe:active:scale-[0.97]',
        'disabled:pointer-events-none disabled:text-fg-4 [&_svg]:size-3.5 [&_svg]:shrink-0',
        FOCUS_RING,
        pressed
          ? 'border-fg-2 bg-surface-4 font-medium text-fg-1'
          : 'border-border-strong bg-surface-1 text-fg-2 hover:bg-surface-3 hover:text-fg-1',
        className,
      )}
      {...props}
    >
      {children}
    </button>
  );
}

/** A small titled group of the left column; the title is a plain label, not a heading level. */
export function ReplayGroup({ title, aside, children, className }: { title: string; aside?: ReactNode; children: ReactNode; className?: string }): ReactNode {
  return (
    <section className={cn('flex flex-col gap-1.5', className)} aria-label={title}>
      <div className="flex items-center justify-between gap-2">
        <h3 className="text-meta font-medium text-fg-3">{title}</h3>
        {aside}
      </div>
      {children}
    </section>
  );
}

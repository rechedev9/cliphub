'use client';

import type { ReactNode } from 'react';
import { ChevronDown, ChevronRight, Trash2 } from 'lucide-react';
import type { FinderCount } from '@/components/patterns/finder-frame';
import { cn } from '@/lib/utils';

/** The colour of a panel header: the finders tell apart at a glance, as in EDGE. */
export type PanelTone = 'dark' | 'mint' | 'blue';

const HEADER: Record<PanelTone, { open: string; dot: string }> = {
  dark: { open: 'bg-surface-5 text-fg-1', dot: 'bg-fg-1' },
  mint: { open: 'bg-primary text-primary-foreground', dot: 'bg-primary' },
  blue: { open: 'bg-side-ct text-primary-foreground', dot: 'bg-side-ct' },
};

/** "127/130 RONDAS", the rounds the panel keeps over the rounds its scope reads. */
function counterText(count: FinderCount | null | undefined): string {
  return count === null || count === undefined ? '' : `${count.matched}/${count.total} rondas`;
}

/**
 * One finder of the canvas: a header with a dot, its title, the live round counter, a trash button and a
 * chevron; the open one carries its controls in `children`.
 */
export function CanvasPanel({
  title,
  tone,
  count,
  open,
  removable,
  onOpen,
  onRemove,
  children,
}: {
  title: string;
  tone: PanelTone;
  count: FinderCount | null | undefined;
  open: boolean;
  /** The last panel left cannot be removed. */
  removable: boolean;
  onOpen: () => void;
  onRemove: () => void;
  children: ReactNode;
}): ReactNode {
  const Chevron = open ? ChevronDown : ChevronRight;
  // overflow-hidden drops a flex item's minimum height to 0, so without shrink-0 the column clips the panel instead of scrolling.
  return (
    <section aria-label={title} className="shrink-0 overflow-hidden rounded-md border border-border-strong bg-surface-2">
      <header className={cn('flex items-center gap-1.5 pr-1', open ? HEADER[tone].open : 'text-fg-2')}>
        <button
          type="button"
          aria-expanded={open}
          onClick={onOpen}
          className="flex min-w-0 flex-1 items-center gap-2 px-2 py-1.5 text-left outline-none focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-ring"
        >
          <span aria-hidden className={cn('size-2 shrink-0 rounded-full', HEADER[tone].dot)} />
          <span className="truncate text-body-sm font-semibold">{title}</span>
          <span className="ml-auto text-meta font-medium tracking-wider whitespace-nowrap uppercase tabular-nums opacity-80">{counterText(count)}</span>
          <Chevron aria-hidden className="size-3.5 shrink-0" />
        </button>
        <button
          type="button"
          aria-label={`Quitar el panel ${title}`}
          title="Quitar el panel"
          disabled={!removable}
          onClick={onRemove}
          className="grid size-6 shrink-0 place-items-center rounded-sm opacity-70 outline-none hover:opacity-100 focus-visible:outline-2 focus-visible:outline-ring disabled:pointer-events-none disabled:opacity-30"
        >
          <Trash2 aria-hidden className="size-3.5" />
        </button>
      </header>
      {open ? <div className="border-t border-border p-2">{children}</div> : null}
    </section>
  );
}

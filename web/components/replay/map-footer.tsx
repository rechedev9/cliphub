'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** Where the demo of the round on screen is described: its date, when the page knows it, and its record. */
export type DemoLink = { date: string | null; href: string };

/**
 * The strip under the map: Detalles de la demo as a pill, centred, and the map's zoom buttons at its right,
 * rendered here (`zoom`) or portalled into `zoomSlot`. One fixed height, so every viewer leaves the map the same room.
 */
export function MapFooter({ demo, zoomSlot, zoom, marks }: { demo: DemoLink | null; zoomSlot?: (element: HTMLDivElement | null) => void; zoom?: ReactNode; marks?: ReactNode }): ReactNode {
  return (
    <div className="relative flex h-10 shrink-0 items-center justify-center border-t border-border-subtle bg-surface-1 px-2">
      {marks === undefined ? null : <div className="absolute left-2">{marks}</div>}
      {demo === null ? null : (
        <Link
          href={demo.href}
          className={cn('flex min-w-0 items-center gap-2 rounded-sm border border-border-strong bg-surface-2 px-2.5 py-1 text-meta text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1', FOCUS_RING)}
        >
          {demo.date === null ? null : <span className="tabular-nums">{demo.date}</span>}
          <span className="truncate font-semibold text-fg-1">Detalles de la demo</span>
        </Link>
      )}
      <div ref={zoomSlot} className="absolute right-2">
        {zoom}
      </div>
    </div>
  );
}

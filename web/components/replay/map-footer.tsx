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
 * The side columns keep their content and the pill truncates, so a narrow viewer never stacks them.
 */
export function MapFooter({ demo, zoomSlot, zoom, marks }: { demo: DemoLink | null; zoomSlot?: (element: HTMLDivElement | null) => void; zoom?: ReactNode; marks?: ReactNode }): ReactNode {
  return (
    <div className="@container grid h-10 shrink-0 grid-cols-[1fr_minmax(0,auto)_1fr] items-center gap-2 border-t border-border-subtle bg-surface-1 px-2">
      {marks === undefined ? null : <div className="col-start-1 justify-self-start">{marks}</div>}
      {demo === null ? null : (
        <Link
          href={demo.href}
          className={cn(
            'col-start-2 flex min-w-0 max-w-full items-center gap-2 justify-self-center rounded-sm border border-border-strong bg-surface-2 px-2.5 py-1 text-meta text-fg-2 outline-none hover:bg-surface-3 hover:text-fg-1',
            FOCUS_RING,
          )}
        >
          {demo.date === null ? null : <span className="tabular-nums whitespace-nowrap @max-sm:hidden">{demo.date}</span>}
          <span className="truncate font-semibold text-fg-1">Detalles de la demo</span>
        </Link>
      )}
      <div ref={zoomSlot} className="col-start-3 justify-self-end">
        {zoom}
      </div>
    </div>
  );
}

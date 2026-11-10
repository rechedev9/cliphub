'use client';

import type { ReactNode } from 'react';
import { Check, TriangleAlert } from 'lucide-react';
import type { SetposNote } from './use-setpos-copies';

/** Says over the map that a setpos was copied, or why not; the live region stays mounted so readers hear it. */
export function SetposNotice({ note }: { note: SetposNote | null }): ReactNode {
  return (
    <div role="status" className="pointer-events-none absolute inset-x-0 bottom-3 z-20 flex justify-center px-3">
      {note === null ? null : (
        <p
          key={note.id}
          className="flex max-w-full items-center gap-1.5 rounded-md border border-border-strong bg-surface-0/90 px-3 py-1.5 text-body-sm font-medium text-fg-1 shadow-[var(--elev-2)] motion-safe:animate-in motion-safe:fade-in motion-safe:slide-in-from-bottom-1"
        >
          {note.ok ? <Check className="size-4 shrink-0 text-success" aria-hidden /> : <TriangleAlert className="size-4 shrink-0 text-destructive" aria-hidden />}
          <span className="flex min-w-0 flex-col">
            <span className="truncate">{note.text}</span>
            {note.command === null ? null : <code className="truncate font-mono text-meta font-normal text-fg-2">{note.command}</code>}
          </span>
        </p>
      )}
    </div>
  );
}

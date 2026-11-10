'use client';

import type { ReactNode } from 'react';
import Link from 'next/link';
import { Copy } from 'lucide-react';
import { RESULTS_HREF } from '@/lib/clips/routes';
import { roundPovLabel } from '@/lib/replay/round-pov';
import type { RoundPovRun } from '@/lib/replay/round-pov';
import type { SetposCopy } from '@/lib/replay/setpos';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/** A small icon button of a roster row or a log line. */
export function RowAction({ label, pressed, onClick, children }: { label: string; pressed: boolean; onClick: () => void; children: ReactNode }): ReactNode {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      aria-pressed={pressed}
      onClick={(event) => {
        event.stopPropagation();
        onClick();
      }}
      className={cn(
        'grid size-6 place-items-center rounded-sm text-fg-3 outline-none hover:bg-surface-4 hover:text-fg-1 [&_svg]:size-3.5',
        FOCUS_RING,
      )}
    >
      {children}
    </button>
  );
}

/** The POV recordings of this visit, under the roster cards. */
export function PovRuns({ runs }: { runs: readonly RoundPovRun[] }): ReactNode {
  return (
    <section aria-label="POV grabados" className="flex flex-col gap-1 border-t border-border-subtle pt-2">
      <h3 className="px-1 text-meta font-semibold text-fg-3">POV grabados</h3>
      <ul className="flex flex-col gap-px">
        {runs.map((run) => (
          <li key={run.key} className="flex flex-col gap-0.5 rounded-sm px-1 py-0.5 hover:bg-surface-3">
            <span className="flex items-center gap-1.5 text-body-sm text-fg-1">
              <span className="min-w-0 flex-1 truncate">
                {run.name} <span className="text-meta tabular-nums text-fg-3">ronda {run.round}</span>
              </span>
              <span role="status" className={cn('text-meta', run.phase === 'failed' ? 'text-destructive' : 'text-fg-2')}>
                {roundPovLabel(run)}
              </span>
            </span>
            {run.phase === 'ready' ? (
              <Link href={RESULTS_HREF} className="text-meta text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1">
                Ver en Mis vídeos
              </Link>
            ) : null}
            {run.error === undefined ? null : <span className="text-meta text-fg-3">{run.error}</span>}
          </li>
        ))}
      </ul>
    </section>
  );
}

/** The setpos lines copied with S or a right-click, newest first, under the roster cards. */
export function SetposList({ setpos, onCopy }: { setpos: readonly SetposCopy[]; onCopy: (entry: SetposCopy) => void }): ReactNode {
  return (
    <section aria-label="Setpos copiados" className="flex flex-col gap-1 border-t border-border-subtle pt-2">
      <h3 className="px-1 text-meta font-semibold text-fg-3">Setpos copiados</h3>
      <ul className="flex flex-col gap-px">
        {setpos.map((entry) => (
          <li key={entry.id} className="flex items-center gap-1.5 rounded-sm px-1 py-0.5 hover:bg-surface-3">
            <span className="min-w-0 flex-1 truncate text-body-sm text-fg-1" title={entry.command}>
              {entry.name} <span className="text-meta tabular-nums text-fg-3">{entry.at}</span>
            </span>
            <RowAction label={`Copiar otra vez el setpos de ${entry.name}`} pressed={false} onClick={() => onCopy(entry)}>
              <Copy aria-hidden />
            </RowAction>
          </li>
        ))}
      </ul>
      <p className="px-1 text-meta text-fg-3">Pégalo en la consola de CS2, con sv_cheats 1.</p>
    </section>
  );
}

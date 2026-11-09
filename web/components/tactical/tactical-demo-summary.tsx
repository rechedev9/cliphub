import type { ReactNode } from 'react';
import { TriangleAlert } from 'lucide-react';
import { RADAR_CALIBRATION_SOURCES, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument } from '@/lib/api/tactical';
import { prettyMapName, timeAgo } from '@/lib/format';
import { FOCUS_RING } from '@/components/ui/button';
import { cn } from '@/lib/utils';

/**
 * What the analysis was computed from, in one line above the replay: the map,
 * both teams with the side they started on, the sampling, and the scan's
 * warnings behind a toggle so they stay one click away without pushing the
 * replay down.
 */
export function TacticalDemoSummary({ doc, generatedAt, actions }: { doc: TacticalDocument; generatedAt: string; actions?: ReactNode }): ReactNode {
  const derived = doc.geometry.calibration.source === RADAR_CALIBRATION_SOURCES.derived;
  const warnings = [
    ...(derived ? ['Este mapa no trae calibración oficial: el encuadre se derivó de las posiciones de esta demo y no es comparable con otra.'] : []),
    ...(doc.warnings ?? []),
  ];
  return (
    <section aria-label="Demo analizada" className="flex min-h-8 flex-wrap items-center gap-x-4 gap-y-1">
      <h2 className="font-display text-body font-bold text-fg-1">{prettyMapName(doc.demo.map)}</h2>
      <p className="flex min-w-0 items-center gap-3 text-body-sm">
        {doc.teams.map((team) => (
          <span key={team.key} className="inline-flex min-w-0 items-center gap-1.5">
            <span className={cn('size-2 shrink-0 rounded-full', team.start_side === TACTICAL_SIDES.ct ? 'bg-side-ct' : 'bg-side-t')} aria-hidden />
            <span className="truncate font-medium text-fg-1">{team.name || team.key}</span>
          </span>
        ))}
      </p>
      <p className="text-meta tabular-nums text-fg-3">
        {doc.rounds.length} rondas{doc.demo.max_rounds > 0 ? ` · MR${Math.round(doc.demo.max_rounds / 2)}` : ''} · {doc.positions.hz} Hz · analizada {timeAgo(generatedAt)}
      </p>
      {warnings.length === 0 ? null : (
        <details className="group relative">
          <summary className={cn('flex h-7 cursor-pointer list-none items-center gap-1.5 rounded-sm px-1.5 text-meta text-fg-2 outline-none hover:bg-surface-3', FOCUS_RING)}>
            <TriangleAlert className="size-3.5" aria-hidden />
            {warnings.length} aviso{warnings.length === 1 ? '' : 's'}
          </summary>
          <ul className="absolute top-8 left-0 z-20 flex w-[min(28rem,80vw)] flex-col gap-1.5 rounded-md border border-border-strong bg-surface-2 p-3 shadow-[var(--elev-4)]">
            {warnings.map((warning) => (
              <li key={warning} className="text-body-sm leading-5 text-fg-2">
                {warning}
              </li>
            ))}
          </ul>
        </details>
      )}
      {actions === undefined ? null : <div className="ml-auto">{actions}</div>}
    </section>
  );
}

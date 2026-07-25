import type { ReactNode } from 'react';
import { Info, TriangleAlert } from 'lucide-react';
import { RADAR_CALIBRATION_SOURCES, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { TacticalDocument, TacticalTeam } from '@/lib/api/tactical';
import { prettyMapName, timeAgo } from '@/lib/format';
import { cn } from '@/lib/utils';

function StatCell({ label, value }: { label: string; value: string }): ReactNode {
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.16em] text-muted-foreground">
        {label}
      </span>
      <span className="truncate font-[family-name:var(--font-mono)] text-sm tabular-nums text-foreground">
        {value}
      </span>
    </div>
  );
}

function TeamChip({ team }: { team: TacticalTeam }): ReactNode {
  const ct = team.start_side === TACTICAL_SIDES.ct;
  return (
    <span className="inline-flex items-center gap-2 truncate">
      <span
        className={cn('size-2 shrink-0 rounded-full', ct ? 'bg-primary' : 'bg-warning')}
        aria-hidden
      />
      <span className="truncate font-[family-name:var(--font-display)] text-sm font-semibold uppercase tracking-[0.04em] text-foreground">
        {team.name || team.key}
      </span>
      <span className="shrink-0 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
        empieza {team.start_side}
      </span>
    </span>
  );
}

/**
 * What the analysis was computed from: the demo's identity, the two rosters and
 * the sampling the replay is drawn at, plus every ambiguity the scan resolved by
 * convention. The warnings are part of the analysis, not decoration, so they are
 * on screen rather than in a log.
 */
export function TacticalDemoSummary({
  doc,
  generatedAt,
}: {
  doc: TacticalDocument;
  generatedAt: string;
}): ReactNode {
  const derived = doc.geometry.calibration.source === RADAR_CALIBRATION_SOURCES.derived;
  const warnings = doc.warnings ?? [];

  return (
    <section className="studio-panel rounded-xl px-5 py-5 sm:px-6" aria-label="Demo analizada">
      <div className="flex flex-col gap-5">
        <div className="flex flex-wrap items-baseline justify-between gap-x-6 gap-y-2">
          <h2 className="font-[family-name:var(--font-display)] text-2xl font-bold uppercase leading-none tracking-tight text-foreground">
            {prettyMapName(doc.demo.map)}
          </h2>
          <span className="font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-muted-foreground">
            analizada {timeAgo(generatedAt)}
          </span>
        </div>

        <div className="flex flex-wrap items-center gap-x-6 gap-y-2">
          {doc.teams.map((team) => (
            <TeamChip key={team.key} team={team} />
          ))}
        </div>

        <div className="grid grid-cols-2 gap-x-6 gap-y-4 border-t border-border/60 pt-4 sm:grid-cols-3 lg:grid-cols-5">
          <StatCell label="Rondas" value={String(doc.rounds.length)} />
          <StatCell label="Formato" value={`MR${Math.round(doc.demo.max_rounds / 2)}`} />
          <StatCell label="Tickrate" value={`${doc.demo.tickrate}`} />
          <StatCell label="Muestreo" value={`${doc.positions.hz} Hz`} />
          <StatCell label="Muestras" value={doc.positions.frame_count.toLocaleString('es-ES')} />
        </div>

        {derived ? (
          <p className="flex items-start gap-2 rounded-md border border-warning/35 bg-warning/8 px-3 py-2 text-[13px] leading-5 text-warning">
            <Info className="mt-0.5 size-4 shrink-0" aria-hidden />
            <span>
              Este mapa no trae calibración oficial: el encuadre se derivó de las posiciones observadas. Es
              estable dentro de esta demo, pero no comparable con otra.
            </span>
          </p>
        ) : null}

        {warnings.length > 0 ? (
          <details className="group rounded-md border border-border/70 bg-background/35">
            <summary className="flex cursor-pointer list-none items-center gap-2 px-3 py-2.5 font-[family-name:var(--font-mono)] text-[11px] uppercase tracking-[0.14em] text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background">
              <TriangleAlert className="size-3.5 text-warning" aria-hidden />
              {warnings.length} aviso{warnings.length === 1 ? '' : 's'} del escaneo
            </summary>
            <ul className="flex flex-col gap-1.5 border-t border-border/70 px-3 py-3">
              {warnings.map((warning) => (
                <li
                  key={warning}
                  className="font-[family-name:var(--font-mono)] text-[12px] leading-5 text-muted-foreground"
                >
                  {warning}
                </li>
              ))}
            </ul>
          </details>
        ) : null}
      </div>
    </section>
  );
}

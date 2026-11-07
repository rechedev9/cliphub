'use client';

import { useId, useState, type ReactNode } from 'react';
import { Radar } from 'lucide-react';
import { LIBRARY_TEAM_KEYS, type LibraryMatch, type LibraryTeamKey } from '@/lib/api/library';
import type { RoundTimeline } from '@/lib/api/metrics';
import { teamByKey, teamLabel } from '@/lib/library/partidas';
import { percentText, ppText } from '@/lib/metrics/advanced';
import { roundBars, winPointText, type RoundBar } from '@/lib/metrics/match-view';
import { secondsText } from '@/lib/reports/scout-sections';
import { Button } from '@/components/ui/button';
import { ToggleGroup, ToggleGroupItem } from '@/components/ui/toggle-group';
import { cn } from '@/lib/utils';

const OUTCOME_VERB: Record<RoundBar['outcome'], string> = { win: 'la ganó', loss: 'la perdió', none: 'acabó sin ganador' };

function barLabel(bar: RoundBar, team: string): string {
  return `Ronda ${bar.round}: ${team} empezó con un ${percentText(bar.chance)} y ${OUTCOME_VERB[bar.outcome]}`;
}

function resultText(bar: RoundBar, team: string): string {
  if (bar.outcome === 'none') return 'sin ganador';
  return bar.outcome === 'win' ? `la gana ${team}` : `la pierde ${team}`;
}

function RoundDetail({ bar, team, names }: { bar: RoundBar; team: string; names: ReadonlyMap<number, string> }): ReactNode {
  const swing = bar.swing;
  const side = swing && swing.delta > 0 ? 'CT' : 'T';
  return (
    <div className="flex min-w-0 flex-[1_1_26rem] flex-col gap-0.5" aria-live="polite">
      <span className="text-body-sm font-semibold text-fg-1">
        Ronda {bar.round} · {resultText(bar, team)}
      </span>
      <span className="text-body-sm text-fg-2">
        {team} empezó con un {percentText(bar.chance)}.{' '}
        {swing ? (
          <>
            La baja que más la movió: <span className="font-medium text-fg-1">{winPointText(swing.point, names)}</span> a los{' '}
            {secondsText(swing.seconds)},{' '}
            <span className={cn('font-semibold', side === 'CT' ? 'text-side-ct' : 'text-side-t')}>
              {side} {ppText(Math.abs(swing.delta))}
            </span>
            .
          </>
        ) : (
          'No hubo bajas en esta ronda.'
        )}
      </span>
    </div>
  );
}

/**
 * One bar per round with a team's win chance when the round starts, green when
 * it won the round. Picking a bar selects that round for the whole tab.
 */
export function MatchWinChart({
  match,
  timelines,
  ctTeams,
  names,
  round,
  onRoundChange,
  curveOpen,
  curveId,
  onCurveToggle,
}: {
  match: LibraryMatch;
  timelines: readonly RoundTimeline[];
  ctTeams: ReadonlyMap<number, LibraryTeamKey>;
  names: ReadonlyMap<number, string>;
  round: number;
  onRoundChange: (round: number) => void;
  /** Whether the selected round's second-by-second curve is open below. */
  curveOpen: boolean;
  curveId: string;
  onCurveToggle: () => void;
}): ReactNode {
  const headingId = useId();
  const [team, setTeam] = useState<LibraryTeamKey>('ct-start');
  const bars = roundBars({ timelines, ctTeams, team });
  const selected = bars.find((bar) => bar.round === round) ?? bars[0];
  if (!selected) return null;
  const teamName = teamLabel(teamByKey(match, team));
  return (
    <section aria-labelledby={headingId} className="studio-panel flex flex-col gap-4 p-5">
      <div className="flex flex-wrap items-center gap-3">
        <div className="flex flex-[1_1_22.5rem] flex-col gap-0.5">
          <h2 id={headingId} className="text-title font-semibold text-fg-1">
            Probabilidad de ganar cada ronda
          </h2>
          <p className="text-body-sm text-fg-2">
            La de {teamName} al empezar la ronda, según el equipamiento de los dos equipos y el mapa.
          </p>
        </div>
        <ToggleGroup
          type="single"
          variant="filter"
          spacing={2}
          aria-label="Equipo"
          value={team}
          onValueChange={(next: string) => setTeam(LIBRARY_TEAM_KEYS.find((key) => key === next) ?? team)}
        >
          {LIBRARY_TEAM_KEYS.map((key) => (
            <ToggleGroupItem key={key} value={key}>
              {teamLabel(teamByKey(match, key))}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </div>

      <div className="overflow-x-auto">
        <div className="relative flex gap-1 pt-1 pr-10" style={{ minWidth: `${bars.length * 1.25 + 2.5}rem` }}>
          <span aria-hidden className="pointer-events-none absolute inset-x-0 top-21 border-t border-dashed border-border-strong" />
          <span aria-hidden className="absolute top-21 right-0 -translate-y-full pb-0.5 text-meta text-fg-3">
            50 %
          </span>
          {bars.map((bar) => {
            const active = bar.round === selected.round;
            const label = barLabel(bar, teamName);
            return (
              <button
                key={bar.round}
                type="button"
                aria-pressed={active}
                aria-label={label}
                title={label}
                onClick={() => onRoundChange(bar.round)}
                className={cn(
                  'group flex min-w-0 flex-1 flex-col items-center gap-1.5 rounded-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  bar.halfStart && 'ml-3',
                )}
              >
                <span className="flex h-40 w-full items-end justify-center">
                  <span
                    className={cn(
                      'min-h-1 w-full max-w-7.5 rounded-t-sm transition-opacity group-hover:opacity-85',
                      bar.outcome === 'win' ? 'bg-outcome-win' : 'bg-fg-4',
                      active && 'ring-2 ring-fg-1 ring-offset-2 ring-offset-surface-2',
                    )}
                    style={{ height: `${Math.round(bar.chance * 100)}%` }}
                  />
                </span>
                <span className={cn('text-meta', active ? 'font-bold text-fg-1' : 'text-fg-3')}>{bar.round}</span>
              </button>
            );
          })}
        </div>
      </div>
      <div className="flex flex-wrap gap-x-4.5 gap-y-2 text-meta text-fg-2">
        <span className="inline-flex items-center gap-1.5">
          <span aria-hidden className="size-3 rounded-sm bg-outcome-win" />
          Ronda ganada
        </span>
        <span className="inline-flex items-center gap-1.5">
          <span aria-hidden className="size-3 rounded-sm bg-fg-4" />
          Ronda perdida
        </span>
        <span className="text-fg-3">La altura es la probabilidad al empezar</span>
      </div>

      <div className="flex flex-wrap items-center gap-x-5 gap-y-3 rounded-md border border-border-subtle bg-surface-1 px-4 py-3.5">
        <RoundDetail bar={selected} team={teamName} names={names} />
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-expanded={curveOpen}
          aria-controls={curveOpen ? curveId : undefined}
          onClick={onCurveToggle}
        >
          <Radar aria-hidden />
          {curveOpen ? 'Ocultar segundo a segundo' : 'Ver segundo a segundo'}
        </Button>
      </div>
    </section>
  );
}

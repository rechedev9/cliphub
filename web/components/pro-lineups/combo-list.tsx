'use client';

import type { ReactNode } from 'react';
import { ChevronLeft, ChevronRight, Crosshair, Play } from 'lucide-react';
import { pct, practiceSpot, PRO_LINEUP_KINDS, roundClock, techniqueLabel, type ProCombo, type ProLineup } from '@/lib/api/pro-lineups';
import { grenadeIcon } from '@/lib/analysis/icons';
import { spotLabel } from '@/lib/pro-lineups-view';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { cn } from '@/lib/utils';
import { ClipVideo } from './throw-clip';

/** "2 humos, 1 flash" as icons: what a combo throws at a glance. */
function Makeup({ lineups }: { lineups: readonly ProLineup[] }): ReactNode {
  return (
    <span className="flex items-center gap-2">
      {PRO_LINEUP_KINDS.map((kind) => {
        const count = lineups.filter((l) => l.kind === kind).length;
        if (count === 0) return null;
        return (
          <span key={kind} className="inline-flex items-center gap-0.5 text-meta text-fg-2 tabular-nums">
            <span aria-hidden className="flex" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
              <EquipmentIcon name={grenadeIcon(kind)} height={14} />
            </span>
            {count}
          </span>
        );
      })}
    </span>
  );
}

function comboFacts(combo: ProCombo): string {
  return `${combo.side} · ${combo.rounds.recent} rondas · ${combo.team_count} equipos · ${pct(combo.round_win)} ganadas`;
}

/** The combos the filters keep, most rounds first; picking one draws it on the map. */
export function ComboList({
  combos,
  lineupsOf,
  video,
  hover,
  onHover,
  onOpen,
}: {
  combos: readonly ProCombo[];
  lineupsOf: (combo: ProCombo) => ProLineup[];
  video: boolean;
  hover: string | null;
  onHover: (id: string | null) => void;
  onOpen: (combo: ProCombo) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-2">
      <h2 className="text-body font-semibold text-fg-1 tabular-nums">
        {combos.length} {combos.length === 1 ? 'combo' : 'combos'}
      </h2>
      {combos.length === 0 ? (
        <p className="rounded-lg border border-border bg-surface-1 px-3 py-3 text-body-sm text-fg-2">
          {video ? 'Ningún combo con vídeo en este lado. Quita «Con vídeo» para ver todos.' : 'Aún no hay combos que varios equipos repitan en este lado.'}
        </p>
      ) : (
        <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
          {combos.map((combo) => (
            <li key={combo.id} onPointerEnter={() => onHover(combo.id)} onPointerLeave={() => onHover(null)}>
              <button
                type="button"
                onClick={() => onOpen(combo)}
                onFocus={() => onHover(combo.id)}
                onBlur={() => onHover(null)}
                className={cn(
                  'flex min-h-11 w-full items-start gap-3 px-3 py-2.5 text-left transition-[background-color] hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none',
                  hover === combo.id && 'bg-surface-2',
                )}
              >
                <span className="flex min-w-0 flex-1 flex-col gap-1">
                  <span className="text-body-sm font-medium text-fg-1">{combo.title}</span>
                  <span className="flex flex-wrap items-center gap-x-3 gap-y-1">
                    <Makeup lineups={lineupsOf(combo)} />
                    <span className="text-meta text-fg-2 tabular-nums">{comboFacts(combo)}</span>
                  </span>
                </span>
                {!video && combo.clip ? <Play aria-label="Con vídeo" className="mt-1 size-3.5 shrink-0 text-fg-2" /> : null}
                <ChevronRight aria-hidden className="mt-1 size-4 shrink-0 text-fg-3" />
              </button>
            </li>
          ))}
        </ul>
      )}
    </div>
  );
}

/** One combo: its clip, its throws in order and practice for all of them at once. */
export function ComboOpen({
  map,
  combo,
  lineups,
  hover,
  onHover,
  onLineup,
  onBack,
}: {
  map: string;
  combo: ProCombo;
  lineups: readonly ProLineup[];
  hover: string | null;
  onHover: (id: string | null) => void;
  onLineup: (lineup: ProLineup) => void;
  onBack: () => void;
}): ReactNode {
  const practice = usePracticeLaunch(map);
  const recorded = combo.examples.find((example) => example.clip !== undefined && example.clip === combo.clip);
  const teams = combo.teams.slice(0, 3).map((team) => team.name).join(', ');
  return (
    <div className="flex flex-col gap-3">
      <Button type="button" variant="ghost" size="xs" className="-ml-2 self-start" onClick={onBack}>
        <ChevronLeft aria-hidden />
        Combos
      </Button>
      <div>
        <h2 className="text-body font-semibold text-fg-1">{combo.title}</h2>
        <p className="text-meta text-fg-2 tabular-nums">
          {comboFacts(combo)} · empieza a {roundClock(combo.clock)}
        </p>
        {teams === '' ? null : <p className="text-meta text-fg-2">Lo tiran {teams}</p>}
      </div>
      {combo.clip === undefined ? null : (
        <figure className="flex flex-col gap-1.5">
          <ClipVideo clip={combo.clip} label={`Combo ${combo.title}`} />
          {recorded === undefined ? null : (
            <figcaption className="text-meta text-fg-2">
              {recorded.team} · {recorded.event ? `${recorded.event} · ` : ''}R{recorded.round}. Cada lanzamiento, uno tras otro, desde los ojos de quien lo tira.
            </figcaption>
          )}
        </figure>
      )}
      <ol className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
        {lineups.map((lineup, index) => (
          <li key={lineup.id} onPointerEnter={() => onHover(lineup.id)} onPointerLeave={() => onHover(null)}>
            <button
              type="button"
              onClick={() => onLineup(lineup)}
              onFocus={() => onHover(lineup.id)}
              onBlur={() => onHover(null)}
              className={cn(
                'flex min-h-11 w-full items-start gap-3 px-3 py-2.5 text-left transition-[background-color] hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none',
                hover === lineup.id && 'bg-surface-2',
              )}
            >
              <span
                aria-hidden
                className={cn(
                  'flex size-6 shrink-0 items-center justify-center rounded-full text-meta font-semibold text-surface-0 tabular-nums',
                  lineup.side === 'CT' ? 'bg-side-ct' : 'bg-side-t',
                )}
              >
                {index + 1}
              </span>
              <span className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="text-body-sm font-medium text-fg-1">{spotLabel({ kind: lineup.kind, place: lineup.land_place })}</span>
                <span className="text-meta text-fg-2">
                  Desde {lineup.throw_place || 'zona sin nombre'} · {techniqueLabel(lineup)}
                </span>
              </span>
              <ChevronRight aria-hidden className="mt-1 size-4 shrink-0 text-fg-3" />
            </button>
          </li>
        ))}
      </ol>
      {practice.launch === null ? null : (
        <Button type="button" size="sm" className="self-start" disabled={practice.state.kind === 'busy' || lineups.length === 0} onClick={() => practice.launch?.(lineups.map(practiceSpot))}>
          <Crosshair aria-hidden />
          Practicar combo
        </Button>
      )}
      <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
    </div>
  );
}

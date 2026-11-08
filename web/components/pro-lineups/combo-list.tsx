'use client';

import Link from 'next/link';
import { useMemo, useState, type ReactNode } from 'react';
import { Crosshair, Play } from 'lucide-react';
import {
  lineupHref,
  pct,
  practiceSpot,
  roundClock,
  type ProCombo,
  type ProLineup,
  type ProLineupMap,
} from '@/lib/api/pro-lineups';
import type { PracticeSpot } from '@/lib/api/practice';
import type { TacticalSide } from '@/lib/api/tactical';
import { PracticeResult, usePracticeLaunch } from '@/components/patterns/practice-launch';
import { Button } from '@/components/ui/button';
import { ClipVideo } from './throw-clip';

const PAGE = 40;

/** The combos pros run on the map, most rounds first, each with its lineups and a practice button. */
export function ComboList({ data, side }: { data: ProLineupMap; side: TacticalSide | '' }): ReactNode {
  const [shown, setShown] = useState(PAGE);
  const practice = usePracticeLaunch(data.map);
  const byId = useMemo(() => new Map(data.lineups.map((l) => [l.id, l])), [data.lineups]);
  const combos = data.combos.filter((combo) => side === '' || combo.side === side);
  if (combos.length === 0) {
    return <p className="text-body-sm text-fg-2">Aún no hay combos que varios equipos repitan en este lado.</p>;
  }
  return (
    <section aria-label="Combos" className="flex flex-col gap-2">
      <PracticeResult state={practice.state} onDismiss={practice.dismiss} />
      <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
        {combos.slice(0, shown).map((combo) => (
          <ComboRow
            key={combo.id}
            map={data.map}
            combo={combo}
            lineups={combo.lineups.flatMap((id) => byId.get(id) ?? [])}
            busy={practice.state.kind === 'busy'}
            onPractice={(spots) => practice.launch(spots)}
          />
        ))}
      </ul>
      {combos.length > shown ? (
        <Button type="button" variant="ghost" size="sm" className="self-start" onClick={() => setShown((n) => n + PAGE)}>
          Mostrar {Math.min(PAGE, combos.length - shown)} más
        </Button>
      ) : null}
    </section>
  );
}

function ComboRow({
  map,
  combo,
  lineups,
  busy,
  onPractice,
}: {
  map: string;
  combo: ProCombo;
  lineups: ProLineup[];
  busy: boolean;
  onPractice: (spots: PracticeSpot[]) => void;
}): ReactNode {
  const teams = combo.teams.slice(0, 3).map((team) => team.name).join(', ');
  const [watching, setWatching] = useState(false);
  const recorded = combo.examples.find((example) => example.clip !== undefined && example.clip === combo.clip);
  return (
    <li className="flex flex-col gap-2 px-3 py-3">
      <div className="flex flex-wrap items-start gap-2">
        <div className="flex min-w-0 flex-col gap-0.5">
          <span className="text-body-sm font-medium text-fg-1">{combo.title}</span>
          <span className="text-meta text-fg-2 tabular-nums">
            {combo.side} · {combo.rounds.recent} rondas · {combo.team_count} equipos · {pct(combo.round_win)} ganadas · empieza a{' '}
            {roundClock(combo.clock)}
            {teams === '' ? null : ` · ${teams}`}
          </span>
        </div>
        {combo.clip === undefined ? null : (
          <Button type="button" variant="ghost" size="xs" className="ml-auto" aria-expanded={watching} onClick={() => setWatching((w) => !w)}>
            <Play aria-hidden />
            {watching ? 'Ocultar vídeo' : 'Ver vídeo'}
          </Button>
        )}
        <Button
          type="button"
          variant="ghost"
          size="xs"
          className={combo.clip === undefined ? 'ml-auto' : undefined}
          disabled={busy || lineups.length === 0}
          onClick={() => onPractice(lineups.map(practiceSpot))}
        >
          <Crosshair aria-hidden />
          Practicar combo
        </Button>
      </div>
      {watching && combo.clip !== undefined ? (
        <figure className="flex max-w-2xl flex-col gap-1.5">
          <ClipVideo clip={combo.clip} label={`Combo ${combo.title}`} />
          {recorded === undefined ? null : (
            <figcaption className="text-meta text-fg-2">
              {recorded.team} · {recorded.event ? `${recorded.event} · ` : ''}R{recorded.round}. Cada lanzamiento, uno tras otro, desde los ojos de quien lo tira.
            </figcaption>
          )}
        </figure>
      ) : null}
      <ol className="flex flex-col gap-1 pl-1">
        {lineups.map((lineup, index) => (
          <li key={lineup.id} className="flex gap-2 text-body-sm">
            <span className="w-4 shrink-0 text-fg-3 tabular-nums">{index + 1}</span>
            <Link href={lineupHref(map, lineup)} className="text-fg-1 underline-offset-2 hover:underline">
              {lineup.title}
            </Link>
          </li>
        ))}
      </ol>
    </li>
  );
}

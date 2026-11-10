'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { TriangleAlert } from 'lucide-react';
import type { AttackPlayer, TeamAttack } from '@/lib/api/attack';
import { getTeamAttack } from '@/lib/api/library';
import { attackHref, lastMatchText, warningText, windowTramo } from '@/lib/analysis/attack';
import { prepWindowLabel, secondsLabel } from '@/lib/analysis/prep';
import { placeLabel } from '@/lib/tactical-labels';
import { SlashTitle } from '@/components/records/document-page';
import { Button } from '@/components/ui/button';
import { Skeleton } from '@/components/ui/skeleton';
import { PrepSection } from './prep-sections';
import { OtherExecutes, TacticalReads } from './tactical-reads';
import { PlayerCard } from './player-card';
import type { PrepQuery } from './prep-body';

const pct = (value: number): string => `${Math.round(value)} %`;

function PlayerColumn({ attack, player }: { attack: TeamAttack; player: AttackPlayer }): ReactNode {
  return (
    <PlayerCard name={player.name} facts={`${player.rounds} rondas de T`}>
      {player.windows.map((window) => (
        <div key={window.from_seconds} className="flex flex-col gap-0.5">
          <h4 className="text-meta text-fg-3">
            {prepWindowLabel(window)} · quieto {pct(window.pace.static.pct)}, lento {pct(window.pace.slow.pct)}, rápido {pct(window.pace.fast.pct)}
          </h4>
          {window.spots.length === 0 ? (
            <p className="text-body-sm text-fg-3">Sin datos</p>
          ) : (
            <ol className="flex flex-col gap-0.5">
              {window.spots.map((spot) => {
                const href = attackHref(attack, {
                  refs: spot.refs,
                  what: `${player.name} en ${placeLabel(spot.callout)}`,
                  zones: [spot.callout],
                  focus: [player.steamid64],
                  tramo: windowTramo(window),
                });
                return (
                  <li key={spot.callout} className="flex items-baseline justify-between gap-2 text-body-sm">
                    {href === undefined ? (
                      <span className="truncate text-fg-1">{placeLabel(spot.callout)}</span>
                    ) : (
                      <Link href={href} className="truncate text-fg-1 hover:text-primary hover:underline">
                        {placeLabel(spot.callout)}
                      </Link>
                    )}
                    <span className="shrink-0 text-fg-2 tabular-nums">
                      {pct(spot.share.pct)} · llega en {secondsLabel(spot.arrival_seconds)}
                    </span>
                  </li>
                );
              })}
            </ol>
          )}
        </div>
      ))}
    </PlayerCard>
  );
}

/** The T side match prep of a rival on one map; every figure opens its rounds in the Replay with the zones marked. */
export function AttackPrep({ query }: { query: PrepQuery & { matchIds?: readonly string[] } }): ReactNode {
  const { teamId, map, from, to } = query;
  // A saved report reads only its own matches; the keys keep the effect from refiring on a new array.
  const matchKey = (query.matchIds ?? []).join(',');
  const lineupKey = (query.lineup ?? []).join(',');
  const [attack, setAttack] = useState<TeamAttack | null>(null);
  const [failed, setFailed] = useState(false);
  const [attempt, setAttempt] = useState(0);

  useEffect(() => {
    let active = true;
    setAttack(null);
    setFailed(false);
    const list = (key: string): string[] | undefined => (key === '' ? undefined : key.split(','));
    getTeamAttack(teamId, { map, from, to, match_ids: list(matchKey), lineup: list(lineupKey) })
      .then((loaded) => {
        if (active) setAttack(loaded);
      })
      .catch(() => {
        if (active) setFailed(true);
      });
    return () => {
      active = false;
    };
  }, [teamId, map, from, to, matchKey, lineupKey, attempt]);

  if (failed) {
    return (
      <PrepSection title="Ataque en T">
        <div className="flex flex-col items-start gap-2">
          <p role="alert" className="text-body-sm text-destructive">No se pudo leer el ataque de este rival.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        </div>
      </PrepSection>
    );
  }
  if (attack === null) return <Skeleton role="status" aria-label="Leyendo el ataque" className="h-56 w-full" />;
  if (attack.round_count === 0) return null;
  return (
    <>
      <header className="flex flex-wrap items-baseline gap-x-3 gap-y-1 border-t border-border pt-4">
        <SlashTitle>Ataque en T</SlashTitle>
        <p className="text-meta text-fg-2">
          {lastMatchText(attack.days_since_last_match)} · {attack.matches} {attack.matches === 1 ? 'partida' : 'partidas'} · {attack.round_count} rondas de T
        </p>
      </header>
      {attack.warnings.length === 0 ? null : (
        <ul role="note" aria-label="Avisos del ataque" className="flex flex-col gap-1 rounded-md border border-border-strong bg-surface-2 px-3 py-2">
          {attack.warnings.map((warning) => (
            <li key={warning.kind} className="flex items-start gap-2 text-body-sm text-fg-1">
              <TriangleAlert className="mt-0.5 size-4 shrink-0 text-fg-2" aria-hidden />
              {warningText(warning)}
            </li>
          ))}
        </ul>
      )}
      <TacticalReads attack={attack} />
      <PrepSection title="Otras ejecuciones por compra">
        <OtherExecutes attack={attack} />
      </PrepSection>
      <PrepSection title="Jugadores en T" hint="Por tramo, dónde está cada uno, cuándo llega y a qué ritmo se mueve.">
        {attack.players.length === 0 ? (
          <p className="text-body-sm text-fg-2">Estas partidas no guardan posiciones de los jugadores.</p>
        ) : (
          <div className="grid gap-2 @[90rem]/content:grid-cols-2 print:grid-cols-1">
            {attack.players.map((player) => (
              <PlayerColumn key={player.steamid64} attack={attack} player={player} />
            ))}
          </div>
        )}
      </PrepSection>
    </>
  );
}

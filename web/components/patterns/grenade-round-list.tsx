'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { PlayCircle } from 'lucide-react';
import { playLabel, roundPlay, type GrenadeRoundRow } from '@/lib/analysis/grenade-follow-up';
import { GRENADE_KIND_LABEL, grenadeId, spotReplayRefs, throwReplayRef } from '@/lib/analysis/grenade-patterns';
import { grenadeIcon } from '@/lib/analysis/icons';
import { replayHref } from '@/lib/analysis/replay-link';
import { roundClock } from '@/lib/replay/tramo';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';

const PAGE = 60;

type Group = { matchId: string; rows: GrenadeRoundRow[] };

function byMatch(rows: readonly GrenadeRoundRow[]): Group[] {
  const out: Group[] = [];
  for (const row of rows) {
    const last = out.at(-1);
    if (last !== undefined && last.matchId === row.round.match_id) last.rows.push(row);
    else out.push({ matchId: row.round.match_id, rows: [row] });
  }
  return out;
}

/**
 * "Por ronda", as EDGE lists it: each match, its rounds, and one line per throw with the thrower in
 * their colour, the grenade and the clock. The play button opens the Replay at that throw.
 */
export function GrenadeRoundList({ rows, matchLabel, tokenOf }: {
  rows: readonly GrenadeRoundRow[];
  matchLabel: (matchId: string) => string;
  /** The colour token of a thrower, the same as the picker and the radar. */
  tokenOf: (steamid64: string) => string;
}): ReactNode {
  const [shown, setShown] = useState(PAGE);
  if (rows.length === 0) return <p className="text-body-sm text-fg-2">Ninguna ronda con este filtro.</p>;
  return (
    <div className="flex flex-col gap-4">
      {byMatch(rows.slice(0, shown)).map((group) => (
        <section key={group.matchId} aria-label={matchLabel(group.matchId)} className="flex flex-col gap-2">
          <h3 className="truncate text-body-sm font-bold text-fg-1">{matchLabel(group.matchId)}</h3>
          {group.rows.map((row) => {
            const { round } = row;
            const title = `${matchLabel(round.match_id)} · ronda ${round.round}`;
            const [ref] = spotReplayRefs(row.grenades);
            const play = roundPlay(round);
            return (
              <div key={row.key} className="flex flex-col">
                <Link
                  href={replayHref([ref ?? { matchId: round.match_id, round: round.round, tick: round.tick }], title)}
                  className="flex items-baseline gap-2 rounded-sm text-meta outline-none hover:text-fg-1 focus-visible:ring-[3px] focus-visible:ring-ring/50"
                >
                  <span className="font-semibold text-fg-1 tabular-nums">Ronda {round.round}</span>
                  <span className={round.won ? 'text-outcome-win' : 'text-outcome-loss'}>{round.won ? 'Ganada' : 'Perdida'}</span>
                  {play === '' ? null : <span className="truncate text-fg-3">{playLabel(play)}</span>}
                </Link>
                {row.grenades.length === 0 ? (
                  <p className="py-1 text-meta text-fg-3">Sin la granada</p>
                ) : (
                  <ul className="flex flex-col">
                    {row.grenades.map((grenade) => (
                      <li key={grenadeId(grenade)} className="grid grid-cols-[minmax(0,1fr)_auto_3rem_auto] items-center gap-2 py-0.5 text-meta">
                        <span className="truncate font-medium" style={{ color: `var(${tokenOf(grenade.thrower_steamid64)})` }}>
                          {grenade.thrower_name}
                        </span>
                        <span className="flex items-center gap-1 text-fg-2">
                          <EquipmentIcon name={grenadeIcon(grenade.kind)} height={12} />
                          {GRENADE_KIND_LABEL[grenade.kind]}
                        </span>
                        <span className="text-right text-fg-2 tabular-nums">{roundClock(grenade.seconds)}</span>
                        <Button asChild variant="outline" size="icon-xs">
                          <Link href={replayHref([throwReplayRef(grenade)], title)} aria-label={`Ver en el Replay ${GRENADE_KIND_LABEL[grenade.kind]} de ${grenade.thrower_name}`} title="Ver en el Replay">
                            <PlayCircle aria-hidden />
                          </Link>
                        </Button>
                      </li>
                    ))}
                  </ul>
                )}
              </div>
            );
          })}
        </section>
      ))}
      {rows.length > shown ? (
        <Button type="button" variant="ghost" size="sm" className="self-start" onClick={() => setShown((count) => count + PAGE)}>
          Mostrar {Math.min(PAGE, rows.length - shown)} más
        </Button>
      ) : null}
    </div>
  );
}

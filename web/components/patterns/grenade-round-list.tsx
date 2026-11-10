'use client';

import { Fragment, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { playLabel, roundPlay, type GrenadeRoundRow } from '@/lib/analysis/grenade-follow-up';
import { GRENADE_KIND_LABEL, grenadeId, spotReplayRefs, throwReplayRef } from '@/lib/analysis/grenade-patterns';
import { replayHref } from '@/lib/analysis/replay-link';
import { roundClock } from '@/lib/replay/tramo';
import { Button } from '@/components/ui/button';

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

/** "Por ronda": every round the grenades were thrown in (or, inverted, not thrown), by match; a click opens it at the throw. */
export function GrenadeRoundList({ rows, matchLabel }: { rows: readonly GrenadeRoundRow[]; matchLabel: (matchId: string) => string }): ReactNode {
  const [shown, setShown] = useState(PAGE);
  if (rows.length === 0) return <p className="text-body-sm text-fg-2">Ninguna ronda con este filtro.</p>;
  return (
    <div className="flex flex-col gap-3">
      {byMatch(rows.slice(0, shown)).map((group) => (
        <section key={group.matchId} aria-label={matchLabel(group.matchId)} className="flex flex-col">
          <h3 className="px-2 text-meta font-medium text-fg-2">
            {matchLabel(group.matchId)} <span className="text-fg-3 tabular-nums">· {group.rows.length}</span>
          </h3>
          <ul className="flex flex-col divide-y divide-border-subtle">
            {group.rows.map((row) => {
              const { round } = row;
              const [ref] = spotReplayRefs(row.grenades);
              const title = `${matchLabel(round.match_id)} · ronda ${round.round}`;
              const play = roundPlay(round);
              return (
                <li key={row.key} className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5 text-meta text-fg-2 hover:bg-surface-3">
                  <Link
                    href={replayHref([ref ?? { matchId: round.match_id, round: round.round, tick: round.tick }], title)}
                    className="flex flex-wrap items-baseline gap-x-2 rounded-sm py-1.5 pl-2 outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50"
                  >
                    <span className="font-medium text-fg-1 tabular-nums">Ronda {round.round}</span>
                    <span className={round.won ? 'text-outcome-win' : 'text-outcome-loss'}>{round.won ? 'Ganada' : 'Perdida'}</span>
                    {play === '' ? null : <span>{playLabel(play)}</span>}
                  </Link>
                  <span className="min-w-0 flex-1 py-1.5 pr-2 text-fg-3">
                    {row.grenades.length === 0
                      ? 'Sin la granada'
                      : row.grenades.map((grenade, index) => (
                          <Fragment key={grenadeId(grenade)}>
                            {index > 0 ? ', ' : null}
                            <Link
                              href={replayHref([throwReplayRef(grenade)], title)}
                              className="rounded-sm underline-offset-2 outline-none hover:text-fg-1 hover:underline focus-visible:ring-[3px] focus-visible:ring-ring/50"
                            >
                              {GRENADE_KIND_LABEL[grenade.kind]} de {grenade.thrower_name} <span className="tabular-nums">{roundClock(grenade.seconds)}</span>
                            </Link>
                          </Fragment>
                        ))}
                  </span>
                </li>
              );
            })}
          </ul>
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

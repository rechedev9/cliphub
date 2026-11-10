'use client';

import { useState, type ReactNode } from 'react';
import Link from 'next/link';
import { Play } from 'lucide-react';
import type { MovementRow } from '@/lib/analysis/pattern-canvas';
import { focusedReplayHref } from '@/lib/analysis/replay-link';
import { roundClock } from '@/lib/replay/tramo';
import { Button } from '@/components/ui/button';

const PAGE = 40;

type Group = { matchId: string; rounds: { round: number; rows: MovementRow[] }[] };

function grouped(rows: readonly MovementRow[]): Group[] {
  const out: Group[] = [];
  for (const row of rows) {
    let group = out.at(-1);
    if (group === undefined || group.matchId !== row.matchId) {
      group = { matchId: row.matchId, rounds: [] };
      out.push(group);
    }
    let entry = group.rounds.at(-1);
    if (entry === undefined || entry.round !== row.round) {
      entry = { round: row.round, rows: [] };
      group.rounds.push(entry);
    }
    entry.rows.push(row);
  }
  return out;
}

/** "Primer movimiento": the moment each chosen player first leaves their spot, by match and round, with a play button that opens it. */
export function MovementList({
  rows,
  names,
  tokens,
  matchLabel,
}: {
  rows: readonly MovementRow[];
  names: ReadonlyMap<string, string>;
  tokens: ReadonlyMap<string, string>;
  matchLabel: (matchId: string) => string;
}): ReactNode {
  const [shown, setShown] = useState(PAGE);
  if (rows.length === 0) {
    return (
      <div className="flex flex-col gap-0.5 text-body-sm">
        <strong className="font-semibold text-fg-1">Ninguna ronda cumple el filtro</strong>
        <span className="text-fg-2">Ajusta el tramo, los jugadores o dibuja otra zona.</span>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-3">
      {grouped(rows.slice(0, shown)).map((group) => (
        <section key={group.matchId} aria-label={matchLabel(group.matchId)} className="flex flex-col gap-1">
          <h3 className="px-1 text-meta font-medium text-fg-2">{matchLabel(group.matchId)}</h3>
          {group.rounds.map((entry) => (
            <div key={entry.round} className="flex flex-col">
              <h4 className="px-1 text-meta font-semibold text-fg-1">Ronda {entry.round}</h4>
              <ul className="flex flex-col divide-y divide-border-subtle">
                {entry.rows.map((row) => {
                  const name = names.get(row.steamid64) ?? row.steamid64;
                  return (
                    <li key={row.steamid64} className="flex items-center gap-2 px-1 py-1 text-body-sm hover:bg-surface-3">
                      <span aria-hidden className="size-2.5 shrink-0 rounded-full" style={{ background: `var(${tokens.get(row.steamid64) ?? '--fg-3'})` }} />
                      <span className="min-w-0 flex-1 truncate text-fg-1">{name}</span>
                      <span className="text-fg-2">Primer movimiento</span>
                      <span className="w-9 text-right font-mono tabular-nums text-fg-1">{roundClock(row.seconds)}</span>
                      <Button asChild variant="outline" size="icon-xs" className="rounded-full">
                        <Link
                          href={focusedReplayHref({ refs: [{ matchId: row.matchId, round: row.round, tick: row.tick }], title: `${matchLabel(row.matchId)} · ronda ${row.round}`, steamId: row.steamid64 })}
                          aria-label={`Ver la ronda ${row.round} con ${name}`}
                        >
                          <Play aria-hidden />
                        </Link>
                      </Button>
                    </li>
                  );
                })}
              </ul>
            </div>
          ))}
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

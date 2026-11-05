'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getBaseline, getLeaderboard, getLibraryPlayerRole, type LibraryRosterEntry } from '@/lib/api/library';
import type { Baseline, BaselineMetric, LeaderboardRow } from '@/lib/api/baseline';
import { METRIC_INFO, baselineValue, referenceLabel } from '@/lib/analysis/reference';
import { playerHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { ROLE_LABEL } from '@/lib/library/partidas';
import { PlayerAvatar } from '@/components/players/player-avatar';
import { Skeleton } from '@/components/ui/skeleton';
import { StatCard } from '@/components/ui/stat-card';

/** The numbers each roster card shows, compared with the library. */
const CARD_METRICS: readonly BaselineMetric[] = ['kpr', 'adr', 'kast'];
/** The leaderboard is the only per-player source of these numbers; its cap bounds the lookup. */
const LOOKUP_LIMIT = 500;

type Values = ReadonlyMap<string, Partial<Record<BaselineMetric, LeaderboardRow>>>;
type Loaded = { baseline: Baseline; values: Values };

async function loadValues(): Promise<Loaded> {
  const [baseline, ...boards] = await Promise.all([
    getBaseline({ scope: 'library' }),
    ...CARD_METRICS.map((metric) => getLeaderboard({ metric, scope: 'library', min_rounds: 1, limit: LOOKUP_LIMIT })),
  ]);
  const values = new Map<string, Partial<Record<BaselineMetric, LeaderboardRow>>>();
  for (const board of boards) {
    for (const row of board.players) values.set(row.steamid64, { ...values.get(row.steamid64), [board.metric]: row });
  }
  return { baseline, values };
}

/** The roster as one card per player, most recent first: role and key numbers against the library. */
export function TeamRoster({ roster }: { roster: readonly LibraryRosterEntry[] }): ReactNode {
  const [data, setData] = useState<Loaded | null | 'failed'>(null);
  const [roles, setRoles] = useState<ReadonlyMap<string, string>>(new Map());
  const ids = roster.map((entry) => entry.steamid64).join(',');

  useEffect(() => {
    let active = true;
    loadValues()
      .then((next) => {
        if (active) setData(next);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    let active = true;
    for (const steamId of ids === '' ? [] : ids.split(',')) {
      getLibraryPlayerRole(steamId)
        .then((role) => {
          if (active) setRoles((current) => new Map(current).set(steamId, ROLE_LABEL[role.role]));
        })
        .catch(() => undefined);
    }
    return () => {
      active = false;
    };
  }, [ids]);

  if (roster.length === 0) return <p className="text-body-sm text-fg-2">Sin jugadores en las partidas de este equipo.</p>;
  const sorted = [...roster].sort((a, b) => Date.parse(b.last_played_at) - Date.parse(a.last_played_at));
  return (
    <div className="flex flex-col gap-3">
      {data === 'failed' ? (
        <p role="alert" className="text-body-sm text-destructive">
          No se pudieron leer las cifras de los jugadores. Comprueba que ClipHub Studio sigue abierto.
        </p>
      ) : null}
      <div className="grid grid-cols-[repeat(auto-fill,minmax(20rem,1fr))] gap-3">
        {sorted.map((entry) => (
          <RosterCard key={entry.steamid64} entry={entry} role={roles.get(entry.steamid64)} data={data} />
        ))}
      </div>
      {data !== null && data !== 'failed' ? (
        <p className="measure-read text-meta text-fg-3">
          Cifras de cada jugador en todas sus partidas de la biblioteca, no solo con este equipo. La media es la de los jugadores con{' '}
          {data.baseline.min_rounds} rondas o más.
        </p>
      ) : null}
    </div>
  );
}

function RosterCard({
  entry,
  role,
  data,
}: {
  entry: LibraryRosterEntry;
  role: string | undefined;
  data: Loaded | null | 'failed';
}): ReactNode {
  const name = entry.nick || entry.steamid64;
  const own = data !== null && data !== 'failed' ? data.values.get(entry.steamid64) : undefined;
  const rounds = own ? Object.values(own)[0]?.rounds : undefined;
  return (
    <article aria-label={name} className="studio-panel flex flex-col overflow-hidden">
      <header className="flex items-center gap-3 px-4 py-3">
        <PlayerAvatar nickname={name} playerID={entry.steamid64} size={40} />
        <div className="flex min-w-0 flex-col">
          <Link href={playerHref(entry.steamid64)} className="truncate text-body font-semibold text-fg-1 hover:text-primary hover:underline">
            {name}
          </Link>
          <span className="text-meta text-fg-2">
            {role ?? 'Rol sin leer'} · {entry.matches === 1 ? '1 partida' : `${entry.matches} partidas`} · última {formatShortDate(entry.last_played_at)}
          </span>
        </div>
      </header>
      {data === null ? (
        <Skeleton className="mx-4 mb-3 h-20" />
      ) : (
        <dl className="grid grid-cols-3 gap-px border-t border-border-subtle bg-border-subtle">
          {CARD_METRICS.map((metric) => {
            const info = METRIC_INFO[metric];
            const reference = data === 'failed' ? null : baselineValue(data.baseline, metric);
            return (
              <StatCard
                key={metric}
                label={metric === 'kpr' ? 'Bajas/ronda' : info.label}
                value={own?.[metric]?.value ?? null}
                format={info.format}
                higherIsBetter={info.higherIsBetter}
                epsilon={info.epsilon}
                comparison={reference === null ? undefined : { value: reference, label: referenceLabel('library') }}
                className="px-3"
              />
            );
          })}
        </dl>
      )}
      {rounds !== undefined ? <p className="border-t border-border-subtle px-4 py-2 text-meta text-fg-3">{rounds} rondas en la biblioteca</p> : null}
    </article>
  );
}

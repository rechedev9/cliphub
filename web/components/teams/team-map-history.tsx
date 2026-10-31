'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getLibraryTeamMaps, type LibraryMatch } from '@/lib/api/library';
import type { SpatialMatchRef } from '@/lib/api/spatial';
import type { MapCount, MapRecord, SideRecord, TeamMaps, VetoCount } from '@/lib/api/veto';
import { librarySeriesHref, matchesHref } from '@/lib/clips/routes';
import { formatShortDate, prettyMapName } from '@/lib/format';
import { percentLabel } from '@/lib/library/partidas';
import { countOf } from '@/lib/library/veto';
import { RelatedList } from '@/components/records/related-list';
import { EvidenceChips } from '@/components/tactical/spatial/spatial-controls';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const VETO_COLUMNS: readonly { key: keyof MapRecord['veto']; label: string }[] = [
  { key: 'picked', label: 'Picks' },
  { key: 'banned', label: 'Bans' },
  { key: 'opponent_picked', label: 'Pick rival' },
  { key: 'opponent_banned', label: 'Ban rival' },
  { key: 'decider', label: 'Decider' },
];

function matchRefs(matches: readonly LibraryMatch[]): ReadonlyMap<string, SpatialMatchRef> {
  return new Map(matches.map((m) => [m.id, { match_id: m.id, map: m.map, played_at: m.played_at }]));
}

function SideCell({ side, refs }: { side: SideRecord | undefined; refs: ReadonlyMap<string, SpatialMatchRef> }): ReactNode {
  if (!side || side.rounds === 0) return <span className="text-fg-3">Sin rondas</span>;
  return (
    <span className="flex flex-col gap-1">
      <span>
        {countOf(side.won.count, side.rounds)} <span className="text-fg-3">({percentLabel(side.won.count, side.rounds)})</span>
      </span>
      <EvidenceChips examples={side.won.examples.slice(0, 3)} matches={refs} />
    </span>
  );
}

function PistolCell({ won, lost, refs }: { won: MapCount; lost: MapCount; refs: ReadonlyMap<string, SpatialMatchRef> }): ReactNode {
  const total = won.count + lost.count;
  if (total === 0) return <span className="text-fg-3">Sin pistolas</span>;
  return (
    <span className="flex flex-col gap-1">
      <span>{countOf(won.count, total)}</span>
      <EvidenceChips examples={[...won.examples, ...lost.examples].slice(0, 4)} matches={refs} />
    </span>
  );
}

function VetoCell({ count }: { count: VetoCount }): ReactNode {
  if (count.count === 0) return <span className="text-fg-3">0</span>;
  return (
    <span className="flex flex-wrap items-center gap-1">
      <span>{count.count}</span>
      {[...new Set(count.series_ids)].slice(0, 3).map((id, index) => (
        <Link key={id} href={librarySeriesHref(id)} className="rounded-sm border border-border px-1.5 text-meta text-fg-2 hover:border-primary hover:text-primary">
          S{index + 1}
        </Link>
      ))}
    </span>
  );
}

/**
 * A team's history per map: results from the scores, rounds per side and
 * pistols from the tactical documents, and the vetoes of its series. Every
 * count links to the matches, rounds or series behind it.
 */
export function TeamMapHistory({ teamId, matches }: { teamId: string; matches: readonly LibraryMatch[] }): ReactNode {
  const [data, setData] = useState<TeamMaps | null | 'failed'>(null);

  useEffect(() => {
    let active = true;
    setData(null);
    getLibraryTeamMaps(teamId)
      .then((next) => {
        if (active) setData(next);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId]);

  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el historial de mapas. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo el historial de mapas" className="flex flex-col gap-3">
        <Skeleton className="h-64 w-full" />
      </div>
    );
  }
  const refs = matchRefs(matches);
  const columns = ['Partidas', 'V-D-E', '% rondas', 'CT', 'T', 'Pistola CT', 'Pistola T', ...VETO_COLUMNS.map((c) => c.label), 'Última'];
  return (
    <div className="flex flex-col gap-3">
      <RelatedList
        title="Historial por mapa"
        count={data.maps.length}
        viewAll={{ href: matchesHref({ team: teamId }), label: 'Ver sus partidas en Partidas' }}
      >
        {data.maps.length === 0 ? (
          <p className="px-4 py-3 text-body-sm text-fg-2">Sin partidas ni vetos todavía.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Historial por mapa" className="w-full border-collapse text-left text-body-sm">
              <thead className="bg-surface-2 text-fg-3">
                <tr className="border-b border-border">
                  <th scope="col" className="h-9 px-3 font-medium">
                    Mapa
                  </th>
                  {columns.map((header) => (
                    <th key={header} scope="col" className="h-9 px-3 font-medium whitespace-nowrap">
                      {header}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {data.maps.map((record) => {
                  const ct = record.sides.find((s) => s.side === 'CT');
                  const t = record.sides.find((s) => s.side === 'T');
                  const rounds = record.rounds_won + record.rounds_lost;
                  return (
                    <tr key={record.map} className="border-b border-border-subtle align-top last:border-b-0">
                      <th scope="row" className="px-3 py-2 text-left font-medium">
                        {record.matches > 0 ? (
                          <Link href={matchesHref({ team: teamId, map: record.map })} className="text-fg-1 hover:text-primary hover:underline">
                            {prettyMapName(record.map)}
                          </Link>
                        ) : (
                          <span className="text-fg-2">{prettyMapName(record.map)}</span>
                        )}
                      </th>
                      <td className="px-3 py-2">{record.matches}</td>
                      <td className="px-3 py-2 whitespace-nowrap">
                        <span className="text-outcome-win">{record.wins}</span>-<span className="text-outcome-loss">{record.losses}</span>-{record.ties}
                      </td>
                      <td className={cn('px-3 py-2', rounds === 0 && 'text-fg-3')}>{percentLabel(record.rounds_won, rounds) || 'Sin rondas'}</td>
                      <td className="px-3 py-2">
                        <SideCell side={ct} refs={refs} />
                      </td>
                      <td className="px-3 py-2">
                        <SideCell side={t} refs={refs} />
                      </td>
                      <td className="px-3 py-2">{ct ? <PistolCell won={ct.pistol_won} lost={ct.pistol_lost} refs={refs} /> : null}</td>
                      <td className="px-3 py-2">{t ? <PistolCell won={t.pistol_won} lost={t.pistol_lost} refs={refs} /> : null}</td>
                      {VETO_COLUMNS.map((column) => (
                        <td key={column.key} className="px-3 py-2">
                          <VetoCell count={record.veto[column.key]} />
                        </td>
                      ))}
                      <td className="px-3 py-2 whitespace-nowrap text-fg-2">{record.last_played_at ? formatShortDate(record.last_played_at) : 'Sin partidas'}</td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </RelatedList>
      <p className="measure-read text-meta text-fg-3">
        Leídas: {data.matches} {data.matches === 1 ? 'partida' : 'partidas'} y {data.series} {data.series === 1 ? 'serie' : 'series'}. Las
        rondas por lado salen del documento táctico de cada partida
        {data.skipped.length > 0 ? `; ${data.skipped.length} ${data.skipped.length === 1 ? 'partida sin análisis no cuenta' : 'partidas sin análisis no cuentan'} en los lados` : ''}. Los picks y bans
        solo cuentan en series con los dos equipos y el veto apuntados.
      </p>
    </div>
  );
}

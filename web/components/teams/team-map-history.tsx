'use client';

import { useEffect, useMemo, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getLibraryTeamMaps, type LibraryMatch } from '@/lib/api/library';
import type { SpatialMatchRef } from '@/lib/api/spatial';
import type { MapCount, MapRecord, SideRecord, TeamMaps, VetoCount } from '@/lib/api/veto';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { librarySeriesHref, matchesHref } from '@/lib/clips/routes';
import { formatShortDate } from '@/lib/format';
import { countOf } from '@/lib/library/veto';
import { MapCard, type MapRate } from '@/components/analysis/map-card';
import { MapLabel } from '@/components/analysis/map-thumb';
import { MapPreferenceScore } from '@/components/analysis/map-preference';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { useMapGeometry } from '@/components/analysis/use-map-geometry';
import { RelatedList } from '@/components/records/related-list';
import { EvidenceChips } from '@/components/tactical/spatial/spatial-controls';
import { Skeleton } from '@/components/ui/skeleton';

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

function sideRate(side: SideRecord | undefined): MapRate {
  return { won: side?.won.count ?? 0, total: side?.rounds ?? 0 };
}

function SideExamples({ label, side, refs }: { label: string; side: SideRecord | undefined; refs: ReadonlyMap<string, SpatialMatchRef> }): ReactNode {
  if (!side || side.won.examples.length === 0) return null;
  return (
    <span className="flex flex-wrap items-center gap-1.5">
      <span className="text-fg-3">{label}</span>
      <EvidenceChips examples={side.won.examples.slice(0, 3)} matches={refs} />
    </span>
  );
}

/**
 * A team's history per map: a card per map with its results and its rounds
 * per side, then pistols and vetoes in a table. Every count links to the
 * matches, rounds or series behind it. The scope's map and period apply.
 */
export function TeamMapHistory({
  teamId,
  matches,
  scope,
  onCounts,
}: {
  teamId: string;
  matches: readonly LibraryMatch[];
  scope: AnalysisScope;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  const [data, setData] = useState<TeamMaps | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setData(null);
    onCounts(null);
    getLibraryTeamMaps(teamId, { from, to })
      .then((next) => {
        if (active) setData(next);
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [teamId, from, to, onCounts]);

  const records = useMemo(
    () => (data === null || data === 'failed' ? [] : data.maps.filter((record) => scope.map === '' || record.map === scope.map)),
    [data, scope.map],
  );
  const played = useMemo(() => records.filter((record) => record.matches > 0), [records]);
  const geometry = useMapGeometry(teamId, played.map((record) => record.map));

  useEffect(() => {
    if (data === null || data === 'failed') return;
    onCounts({
      maps: played.reduce((sum, record) => sum + record.matches, 0),
      rounds: played.reduce((sum, record) => sum + record.rounds_won + record.rounds_lost, 0),
    });
  }, [data, played, onCounts]);

  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudo leer el historial de mapas. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo el historial de mapas" className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-2">
        <Skeleton className="h-44 w-full" />
        <Skeleton className="h-44 w-full" />
        <Skeleton className="h-44 w-full" />
      </div>
    );
  }
  const refs = matchRefs(matches);
  const columns = ['Pistola CT', 'Pistola T', ...VETO_COLUMNS.map((c) => c.label), 'Última'];
  return (
    <div className="flex flex-col gap-4">
      {played.length === 0 ? (
        <p className="text-body-sm text-fg-2">Sin partidas con este filtro. Amplía las fechas o quita el mapa.</p>
      ) : (
        <div className="grid grid-cols-[repeat(auto-fill,minmax(15rem,1fr))] gap-2">
          {played.map((record) => {
            const ct = record.sides.find((s) => s.side === 'CT');
            const t = record.sides.find((s) => s.side === 'T');
            return (
              <MapCard
                key={record.map}
                map={record.map}
                href={matchesHref({ team: teamId, map: record.map })}
                record={{ wins: record.wins, losses: record.losses, ties: record.ties }}
                rounds={{ won: record.rounds_won, total: record.rounds_won + record.rounds_lost }}
                ct={sideRate(ct)}
                t={sideRate(t)}
                geometry={geometry.has(record.map) ? geometry.get(record.map) : undefined}
                footer={
                  <>
                    <span className="flex items-center gap-1.5">
                      <span className="text-fg-3">Preferencia</span>
                      <MapPreferenceScore preference={record.preference} />
                    </span>
                    <SideExamples label="CT ganadas" side={ct} refs={refs} />
                    <SideExamples label="T ganadas" side={t} refs={refs} />
                    {record.last_played_at ? <span className="text-fg-3">Última: {formatShortDate(record.last_played_at)}</span> : null}
                  </>
                }
              />
            );
          })}
        </div>
      )}
      <RelatedList
        title="Pistolas y vetos por mapa"
        count={records.length}
        viewAll={{ href: matchesHref({ team: teamId }), label: 'Ver sus partidas en Partidas' }}
      >
        {records.length === 0 ? (
          <p className="px-4 py-3 text-body-sm text-fg-2">Sin partidas ni vetos todavía.</p>
        ) : (
          <div className="overflow-x-auto">
            <table aria-label="Pistolas y vetos por mapa" className="w-full border-collapse text-left text-body-sm">
              <thead className="bg-surface-2 text-meta text-fg-3">
                <tr className="border-b border-border">
                  <th scope="col" className="sticky left-0 z-10 h-8 bg-surface-2 px-2.5 font-medium">
                    Mapa
                  </th>
                  {columns.map((header) => (
                    <th key={header} scope="col" className="h-8 px-2.5 font-medium whitespace-nowrap">
                      {header}
                    </th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {records.map((record) => {
                  const ct = record.sides.find((s) => s.side === 'CT');
                  const t = record.sides.find((s) => s.side === 'T');
                  return (
                    <tr key={record.map} className="border-b border-border-subtle align-top last:border-b-0">
                      <th scope="row" className="sticky left-0 z-10 bg-surface-2 px-2.5 py-1.5 text-left font-medium text-fg-1">
                        <MapLabel map={record.map} />
                      </th>
                      <td className="px-2.5 py-1.5">{ct ? <PistolCell won={ct.pistol_won} lost={ct.pistol_lost} refs={refs} /> : null}</td>
                      <td className="px-2.5 py-1.5">{t ? <PistolCell won={t.pistol_won} lost={t.pistol_lost} refs={refs} /> : null}</td>
                      {VETO_COLUMNS.map((column) => (
                        <td key={column.key} className="px-2.5 py-1.5">
                          <VetoCell count={record.veto[column.key]} />
                        </td>
                      ))}
                      <td className="px-2.5 py-1.5 whitespace-nowrap text-fg-2">{record.last_played_at ? formatShortDate(record.last_played_at) : 'Sin partidas'}</td>
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

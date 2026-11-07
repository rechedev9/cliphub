'use client';

import { useEffect, useState, type ReactNode } from 'react';
import Link from 'next/link';
import { getPlayerPositions, type LibraryPlayerRole, type LibraryRoleMetric } from '@/lib/api/library';
import type { PlayerMapPositions, PlayerPositions, PlayerSidePositions } from '@/lib/api/team-insights';
import type { EquipmentIconName } from '@/lib/api/game-assets';
import { awpText, rateText, roleSentence } from '@/lib/analysis/positions';
import { replayHref } from '@/lib/analysis/replay-link';
import { scopeDateRange, type AnalysisScope } from '@/lib/analysis/scope';
import { formatDecimal, prettyMapName } from '@/lib/format';
import { ROLE_LABEL, percentLabel, utilityRatioLabel } from '@/lib/library/partidas';
import { MapThumb } from '@/components/analysis/map-thumb';
import type { ScopeCounts } from '@/components/analysis/scope-bar';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { RelatedList } from '@/components/records/related-list';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';

const SIDE_CLASS = { CT: 'text-side-ct', T: 'text-side-t' } as const;

/** The player's role from the whole library, then where they stand per map and side in the page's scope. */
export function PlayerRoles({
  steamId,
  role,
  scope,
  onCounts,
}: {
  steamId: string;
  role: LibraryPlayerRole | null | undefined;
  scope: AnalysisScope;
  onCounts: (counts: ScopeCounts | null) => void;
}): ReactNode {
  return (
    <div className="flex flex-col gap-5">
      <RoleEvidence role={role} />
      <Positions steamId={steamId} scope={scope} onCounts={onCounts} />
    </div>
  );
}

function Positions({ steamId, scope, onCounts }: { steamId: string; scope: AnalysisScope; onCounts: (counts: ScopeCounts | null) => void }): ReactNode {
  const [data, setData] = useState<PlayerPositions | null | 'failed'>(null);
  const [now] = useState(() => Date.now());
  const { map } = scope;
  const { from, to } = scopeDateRange(scope.period, now);

  useEffect(() => {
    let active = true;
    setData(null);
    onCounts(null);
    getPlayerPositions(steamId, { map: map === '' ? undefined : map, from, to })
      .then((report) => {
        if (!active) return;
        setData(report);
        onCounts({
          maps: report.match_count,
          rounds: report.maps.reduce((sum, entry) => sum + entry.sides.reduce((total, side) => total + side.rounds, 0), 0),
        });
      })
      .catch(() => {
        if (active) setData('failed');
      });
    return () => {
      active = false;
    };
  }, [steamId, map, from, to, onCounts]);

  if (data === 'failed') {
    return (
      <p role="alert" className="text-body-sm text-destructive">
        No se pudieron leer sus posiciones. Comprueba que ClipHub Studio sigue abierto.
      </p>
    );
  }
  if (data === null) {
    return (
      <div role="status" aria-label="Leyendo sus posiciones" className="grid grid-cols-[repeat(auto-fill,minmax(20rem,1fr))] gap-3">
        <Skeleton className="h-48 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    );
  }
  if (data.maps.length === 0) {
    return <p className="text-body-sm text-fg-2">Sin partidas analizadas con este filtro. Amplía las fechas o quita el mapa.</p>;
  }
  return (
    <div className="flex flex-col gap-3">
      <div className="grid gap-x-3 gap-y-4 @[90rem]/content:grid-cols-2">
      {data.maps.map((entry) => (
        <MapPositions key={entry.map} entry={entry} minSample={data.min_reliable_sample} />
      ))}
      </div>
      <p className="measure-read text-meta text-fg-3">
        Cada ronda cuenta el sitio donde pasa la mayor parte de sus primeros {data.window_seconds} segundos. En CT es ancla de un sitio si
        está en él en al menos el {Math.round(data.anchor_share * 100)} % de las rondas leídas; si no, rotador. Con menos de{' '}
        {data.min_reliable_sample} rondas la cifra sale en gris.
      </p>
    </div>
  );
}

function MapPositions({ entry, minSample }: { entry: PlayerMapPositions; minSample: number }): ReactNode {
  const name = prettyMapName(entry.map);
  return (
    <section aria-label={name} className="flex flex-col gap-2">
      <h2 className="flex items-center gap-2 text-body font-semibold text-fg-1">
        <MapThumb map={entry.map} className="h-6 w-10" crop={1.4} />
        {name}
        <span className="text-body-sm font-normal text-fg-2">{entry.matches === 1 ? '1 partida' : `${entry.matches} partidas`}</span>
      </h2>
      <div className="grid gap-2 sm:grid-cols-2">
        {entry.sides.map((side) => (
          <SideCard key={side.side} side={side} map={entry.map} minSample={minSample} />
        ))}
      </div>
    </section>
  );
}

function SideCard({ side, map, minSample }: { side: PlayerSidePositions; map: string; minSample: number }): ReactNode {
  const name = prettyMapName(map);
  const low = side.sampled < minSample;
  const awp = awpText(side);
  return (
    <article aria-label={`${name}, ${side.side}`} className="studio-panel flex flex-col">
      <header className="flex flex-col gap-0.5 border-b border-border-subtle px-3 py-2">
        <span className={cn('text-meta font-medium', SIDE_CLASS[side.side])}>{side.side}</span>
        <h3 className={cn('text-body-sm font-semibold', low ? 'text-fg-3' : 'text-fg-1')} title={low ? `Muestra pequeña: ${side.sampled} rondas` : undefined}>
          {roleSentence(side, name)}
        </h3>
        {awp ? (
          <p className="flex items-center gap-1.5 text-body-sm text-fg-2">
            <EquipmentIcon name="awp" height={11} className="text-fg-2" />
            {awp}
          </p>
        ) : null}
      </header>
      {side.places.length === 0 ? (
        <p className="px-3 py-2 text-body-sm text-fg-2">Sin sitios repetidos.</p>
      ) : (
        <ul className="divide-y divide-border-subtle">
          {side.places.map((place) => {
            const refs = place.examples.map((example) => ({ matchId: example.match_id, round: example.round, tick: example.tick }));
            return (
              <li key={place.place} className="flex flex-wrap items-baseline gap-x-3 gap-y-0.5 px-3 py-1 text-body-sm">
                <span className="min-w-0 flex-1 truncate text-fg-1">{place.label || place.place}</span>
                <span className={cn('tabular-nums', place.rounds.reliable ? 'text-fg-2' : 'text-fg-3')}>{rateText(place.rounds)}</span>
                {refs.length > 0 ? (
                  <Link
                    href={replayHref(refs, `${place.label || place.place} en ${name}, ${side.side}`)}
                    className="text-meta text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1"
                  >
                    Ver {refs.length === 1 ? '1 ronda' : `${refs.length} rondas`}
                  </Link>
                ) : null}
              </li>
            );
          })}
        </ul>
      )}
    </article>
  );
}

function RoleEvidence({ role }: { role: LibraryPlayerRole | null | undefined }): ReactNode {
  if (role === undefined) {
    return (
      <div role="status" aria-label="Calculando el rol">
        <Skeleton className="h-40 w-full" />
      </div>
    );
  }
  if (role === null) {
    return (
      <RelatedList title="Rol">
        <p className="px-4 py-3 text-body-sm text-fg-2">No se pudo leer el rol de las partidas de este jugador.</p>
      </RelatedList>
    );
  }
  const { thresholds } = role;
  const ratio = utilityRatioLabel(role.utility, role.team_utility);
  return (
    <RelatedList title={`Rol: ${ROLE_LABEL[role.role]}`}>
      <p className="border-b border-border-subtle px-4 py-3 text-body-sm text-fg-2">
        Leído de {role.rounds} rondas en {role.match_count === 1 ? '1 partida' : `${role.match_count} partidas`}. Se prueba en este orden: AWPer con al menos el{' '}
        {Math.round(thresholds.awp_kill_share * 100)} % de sus kills con AWP, entry si abre el duelo en al menos el{' '}
        {Math.round(thresholds.entry_opening_share * 100)} % de sus rondas de T, soporte si lanza {formatDecimal(thresholds.support_utility_ratio, 2)} veces las granadas por ronda de sus compañeros; si no, rifler. Con menos de {thresholds.min_rounds} rondas no se lee rol.
      </p>
      <dl className="grid divide-y divide-border-subtle @[64rem]/content:grid-cols-3 @[64rem]/content:divide-x @[64rem]/content:divide-y-0">
        <RoleMetricView
          label="Kills con AWP"
          icon="awp"
          metric={role.awp_kills}
          value={percentLabel(role.awp_kills.count, role.awp_kills.total)}
          summary={`${role.awp_kills.count} de ${role.awp_kills.total} kills`}
        />
        <RoleMetricView
          label="Duelos de apertura en T"
          metric={role.t_openings}
          value={percentLabel(role.t_openings.count, role.t_openings.total)}
          summary={`${role.t_openings.count} de ${role.t_openings.total} rondas de T`}
        />
        <RoleMetricView
          label="Granadas frente a sus compañeros"
          icon="smokegrenade"
          metric={role.utility}
          value={ratio === '' ? '' : `${ratio} veces`}
          summary={`${role.utility.count} granadas en ${role.utility.total} rondas`}
          note={`Compañeros: ${role.team_utility.count} granadas en ${role.team_utility.total} rondas.`}
        />
      </dl>
    </RelatedList>
  );
}

function RoleMetricView({
  label,
  metric,
  value,
  summary,
  note,
  icon,
}: {
  label: string;
  icon?: EquipmentIconName;
  metric: LibraryRoleMetric;
  value: string;
  summary: string;
  note?: string;
}): ReactNode {
  return (
    <div className="flex flex-col gap-1 px-3 py-2">
      <dt className="flex items-center gap-1.5 text-meta text-fg-3">
        {icon ? <EquipmentIcon name={icon} height={11} /> : null}
        {label}
      </dt>
      <dd className="flex flex-col gap-1.5">
        <span className="text-title font-semibold text-fg-1">{value || 'Sin dato'}</span>
        <span className="text-meta text-fg-2">{summary}</span>
        {note ? <span className="text-meta text-fg-3">{note}</span> : null}
        {metric.examples.length > 0 ? (
          <Link
            href={replayHref(
              metric.examples.map((example) => ({ matchId: example.match_id, round: example.round, tick: example.tick })),
              label,
            )}
            className="text-meta text-fg-1 underline decoration-fg-4 underline-offset-4 hover:decoration-fg-1"
          >
            Ver {metric.examples.length === 1 ? '1 ronda' : `${metric.examples.length} rondas`}
          </Link>
        ) : null}
      </dd>
    </div>
  );
}

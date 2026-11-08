'use client';

import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { useMemo, useState, type ReactNode } from 'react';
import { Play, Search } from 'lucide-react';
import {
  filterProLineups,
  lineupHref,
  pct,
  PRO_KIND_LABEL,
  PRO_LINEUP_KINDS,
  roundClock,
  type ProLineup,
  type ProLineupKind,
  type ProLineupMap,
} from '@/lib/api/pro-lineups';
import { RADAR_LEVELS, type RadarLevel, type TacticalSide } from '@/lib/api/tactical';
import { grenadeIcon } from '@/lib/analysis/icons';
import { prettyMapName, timeAgo } from '@/lib/format';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, levelForAltitude } from '@/lib/tactical-transform';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { StudioPageHeader } from '@/components/studio/page-header';
import { Button } from '@/components/ui/button';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Input } from '@/components/ui/input';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { Toggle } from '@/components/ui/toggle';
import { cn } from '@/lib/utils';
import { ComboList } from './combo-list';
import { ProLoadProblem } from './lineup-maps';
import { ProLineupRadar } from './pro-lineup-radar';
import { useProLineupMap } from './use-pro-lineups';

type View = 'lineups' | 'combos';
const PAGE = 60;

const SIDES: readonly { value: TacticalSide | ''; label: string }[] = [
  { value: '', label: 'Los dos' },
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
];

function ExplorerSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando lineups" className="grid gap-3 lg:grid-cols-[minmax(0,1.4fr)_minmax(22rem,1fr)]">
      <div className="flex flex-col gap-2">
        <Skeleton className="h-9 w-full max-w-lg" />
        <Skeleton className="aspect-square w-full" />
      </div>
      <div className="flex flex-col gap-2">
        {Array.from({ length: 8 }, (_, index) => (
          <Skeleton key={index} className="h-14 w-full" />
        ))}
      </div>
    </div>
  );
}

/** A map's pro lineups: filters, the landings on the radar and the list most thrown first. */
export function LineupExplorer({ map }: { map: string }): ReactNode {
  const [attempt, setAttempt] = useState(0);
  const load = useProLineupMap(map, attempt);
  const header = <StudioPageHeader title={`Lineups pro de ${prettyMapName(map)}`} description={<Summary load={load} />} />;
  if (load.kind === 'loading') {
    return (
      <>
        {header}
        <ExplorerSkeleton />
      </>
    );
  }
  if (load.kind !== 'ready') {
    return (
      <>
        {header}
        <ProLoadProblem load={load} onRetry={() => setAttempt((n) => n + 1)} map={map} />
      </>
    );
  }
  return (
    <>
      {header}
      <Explorer data={load.data} />
    </>
  );
}

function Summary({ load }: { load: ReturnType<typeof useProLineupMap> }): ReactNode {
  if (load.kind !== 'ready') return 'Granadas que los pros repiten, leídas de sus demos.';
  const d = load.data;
  return (
    <>
      {d.lineups.length} lineups y {d.combos.length} {d.combos.length === 1 ? 'combo' : 'combos'} de {d.maps.recent} mapas pro de los últimos {d.recent_days} días. Actualizado{' '}
      {timeAgo(d.built_at)}
      {d.fresh ? '.' : '; sin conexión con el servidor, es la última copia guardada.'}
    </>
  );
}

function Explorer({ data }: { data: ProLineupMap }): ReactNode {
  const router = useRouter();
  const [view, setView] = useState<View>('lineups');
  const [kinds, setKinds] = useState<ReadonlySet<ProLineupKind>>(() => new Set(PRO_LINEUP_KINDS));
  const [side, setSide] = useState<TacticalSide | ''>('');
  const [insta, setInsta] = useState(false);
  // With recorded clips on the map, the list opens on the lineups you can watch.
  const hasVideo = useMemo(() => data.lineups.some((l) => Boolean(l.clip)), [data.lineups]);
  const [videoChoice, setVideo] = useState<boolean | null>(null);
  const video = videoChoice ?? hasVideo;
  const [text, setText] = useState('');
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [hover, setHover] = useState<string | null>(null);
  const [shown, setShown] = useState(PAGE);
  const geometry = data.geometry;
  const multiLevel = geometry !== undefined && isMultiLevelMap(geometry.calibration);

  const filtered = useMemo(() => filterProLineups(data.lineups, { kinds, side, insta, video, text }), [data.lineups, kinds, side, insta, video, text]);
  const onLevel = useMemo(
    () => (geometry === undefined || !multiLevel ? filtered : filtered.filter((l) => levelForAltitude(geometry.calibration, l.landing[2]) === level)),
    [filtered, geometry, multiLevel, level],
  );
  const counts = useMemo(() => {
    const out = new Map<ProLineupKind, number>();
    for (const l of filterProLineups(data.lineups, { kinds: new Set(PRO_LINEUP_KINDS), side, insta, video, text })) out.set(l.kind, (out.get(l.kind) ?? 0) + 1);
    return out;
  }, [data.lineups, side, insta, video, text]);
  const focus = hover === null ? null : (onLevel.find((l) => l.id === hover) ?? null);

  const toggleKind = (kind: ProLineupKind): void =>
    setKinds((current) => {
      const next = new Set(current);
      if (next.has(kind)) next.delete(kind);
      else next.add(kind);
      return next;
    });

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          label="Vista"
          value={view}
          options={[
            { value: 'lineups', label: `Lineups (${data.lineups.length})` },
            { value: 'combos', label: `Combos (${data.combos.length})` },
          ]}
          onChange={setView}
        />
        <Segmented label="Lado" value={side} options={SIDES} onChange={setSide} />
        {hasVideo ? (
          <Toggle variant="filter" size="sm" pressed={video} onPressedChange={setVideo}>
            <Play aria-hidden className="size-3.5" />
            Con vídeo
          </Toggle>
        ) : null}
      </div>
      {view === 'combos' ? (
        <ComboList data={data} side={side} video={video} />
      ) : (
        <div className="grid items-start gap-3 lg:grid-cols-[minmax(0,1.4fr)_minmax(22rem,1fr)]">
          <section aria-label="Radar" className="flex flex-col gap-2">
            <div className="flex flex-wrap items-center gap-2">
              <div role="group" aria-label="Granadas" className="flex flex-wrap gap-1.5">
                {PRO_LINEUP_KINDS.map((kind) => (
                  <Toggle key={kind} variant="filter" size="sm" pressed={kinds.has(kind)} onPressedChange={() => toggleKind(kind)}>
                    <span aria-hidden className="flex" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
                      <EquipmentIcon name={grenadeIcon(kind)} height={14} />
                    </span>
                    {PRO_KIND_LABEL[kind]}
                    <span className="tabular-nums opacity-80">{counts.get(kind) ?? 0}</span>
                  </Toggle>
                ))}
                <Toggle variant="filter" size="sm" pressed={insta} onPressedChange={setInsta}>
                  Solo instas
                </Toggle>
              </div>
              {multiLevel ? (
                <Segmented
                  label="Piso"
                  value={level}
                  options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))}
                  onChange={setLevel}
                />
              ) : null}
            </div>
            {geometry === undefined ? (
              <p className="text-body-sm text-fg-2">Este mapa aún no trae radar; la lista tiene todos los lineups.</p>
            ) : (
              <ProLineupRadar
                geometry={geometry}
                level={level}
                lineups={onLevel}
                focus={focus}
                onPick={(lineup) => router.push(lineupHref(data.map, lineup))}
                label="Radar con dónde cae cada lineup; pulsa un punto para abrirlo"
              />
            )}
            <p className="text-meta text-fg-3">Cada punto es dónde cae un lineup, más grande cuanto más se tira. Pulsa uno para abrirlo.</p>
          </section>

          <section
            aria-labelledby="lineups-lista"
            className="flex flex-col gap-2 lg:sticky lg:top-[calc(var(--shell-strip-height)+3.5rem)] lg:max-h-[calc(100dvh-var(--shell-strip-height)-4.5rem)] lg:overflow-y-auto"
          >
            <div className="flex items-center gap-2">
              <h2 id="lineups-lista" className="text-body font-semibold text-fg-1 tabular-nums">
                {onLevel.length} {onLevel.length === 1 ? 'lineup' : 'lineups'}
              </h2>
              <label className="relative ml-auto w-full max-w-56">
                <span className="sr-only">Buscar por zona</span>
                <Search aria-hidden className="pointer-events-none absolute top-1/2 left-2.5 size-4 -translate-y-1/2 text-fg-3" />
                <Input value={text} onChange={(event) => setText(event.target.value)} placeholder="Window, Mid, CT Spawn" className="pl-8" />
              </label>
            </div>
            {onLevel.length === 0 ? (
              <p className="text-body-sm text-fg-2">Ningún lineup con estos filtros.</p>
            ) : (
              <ul className="flex flex-col divide-y divide-border rounded-lg border border-border bg-surface-1">
                {onLevel.slice(0, shown).map((lineup) => (
                  <LineupRow key={lineup.id} map={data.map} lineup={lineup} active={lineup.id === hover} onHover={setHover} />
                ))}
              </ul>
            )}
            {onLevel.length > shown ? (
              <Button type="button" variant="ghost" size="sm" className="self-start" onClick={() => setShown((n) => n + PAGE)}>
                Mostrar {Math.min(PAGE, onLevel.length - shown)} más
              </Button>
            ) : null}
          </section>
        </div>
      )}
    </div>
  );
}

function LineupRow({
  map,
  lineup,
  active,
  onHover,
}: {
  map: string;
  lineup: ProLineup;
  active: boolean;
  onHover: (id: string | null) => void;
}): ReactNode {
  return (
    <li onPointerEnter={() => onHover(lineup.id)} onPointerLeave={() => onHover(null)}>
      <Link
        href={lineupHref(map, lineup)}
        onFocus={() => onHover(lineup.id)}
        onBlur={() => onHover(null)}
        className={cn(
          'flex min-h-11 items-start gap-3 px-3 py-2.5 transition-[background-color] hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-none',
          active && 'bg-surface-2',
        )}
      >
        <span aria-hidden className="mt-0.5 flex" style={{ color: `var(${GRENADE_KIND_TOKEN[lineup.kind]})` }}>
          <EquipmentIcon name={grenadeIcon(lineup.kind)} height={16} />
        </span>
        <span className="flex min-w-0 flex-col gap-0.5">
          <span className="text-body-sm font-medium text-fg-1">
            {lineup.title}
            {lineup.clip ? (
              <span className="ml-1.5 inline-flex translate-y-0.5 text-fg-2" title="Con vídeo">
                <Play aria-hidden className="size-3.5" />
                <span className="sr-only">, con vídeo</span>
              </span>
            ) : null}
          </span>
          <span className="text-meta text-fg-2 tabular-nums">
            {lineup.side} · {lineup.throws.recent} lanzamientos · {pct(lineup.round_share)} de rondas · sale a {roundClock(lineup.clock.median)}
          </span>
        </span>
      </Link>
    </li>
  );
}

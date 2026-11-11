'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useEffect, useMemo, useState, type ReactNode } from 'react';
import { Play } from 'lucide-react';
import { PRO_KIND_LABEL, PRO_LINEUP_KINDS, techniqueLabel, type ProCombo, type ProLineup, type ProLineupKind, type ProLineupMap } from '@/lib/api/pro-lineups';
import { RADAR_LEVELS, type TacticalSide } from '@/lib/api/tactical';
import { grenadeIcon } from '@/lib/analysis/icons';
import { prettyMapName, timeAgo } from '@/lib/format';
import { grenadeMeta, type MetaPeriod, type MetaRow } from '@/lib/pro-lineups-meta';
import {
  DEFAULT_QUERY,
  explorerLineups,
  landingSpots,
  plainTitle,
  readExplorerQuery,
  spotLabel,
  spotOf,
  visibleSpots,
  writeExplorerQuery,
  type ExplorerKind,
  type ExplorerQuery,
  type ProSpot,
} from '@/lib/pro-lineups-view';
import { LEVEL_LABEL } from '@/lib/tactical-spatial';
import { isMultiLevelMap, levelForAltitude } from '@/lib/tactical-transform';
import { GRENADE_KIND_TOKEN } from '@/components/patterns/grenade-radar';
import { SlashTitle } from '@/components/records/document-page';
import { EquipmentIcon } from '@/components/ui/equipment-icon';
import { Segmented } from '@/components/ui/segmented';
import { Skeleton } from '@/components/ui/skeleton';
import { Toggle } from '@/components/ui/toggle';
import { ComboList, ComboOpen } from './combo-list';
import { ProLoadProblem } from './lineup-maps';
import { ProLineupRadar, type RadarSpot, type RadarThrow } from './pro-lineup-radar';
import { MetaList } from './meta-list';
import { LineupPreview, SpotLineups, SpotList } from './spot-panel';
import { useProLineupMap } from './use-pro-lineups';

const SIDES = [
  { value: '', label: 'Los dos' },
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
] as const;

const META_SIDES = [
  { value: 'T', label: 'T' },
  { value: 'CT', label: 'CT' },
] as const;
/** The meta draws its top grenades on the radar. */
const META_ON_RADAR = 10;

const KIND_TAB: Record<ExplorerKind, string> = { all: 'Todas', smoke: 'Humos', flash: 'Flashes', molotov: 'Molotovs', he: 'HE' };
const KIND_ORDER: readonly ExplorerKind[] = ['smoke', 'flash', 'molotov', 'he', 'all'];

const PANEL = 'lg:sticky lg:top-[calc(var(--shell-strip-height)+1rem)] lg:max-h-[calc(100dvh-var(--shell-strip-height)-2rem)] lg:overflow-y-auto';

function ExplorerSkeleton(): ReactNode {
  return (
    <div role="status" aria-label="Cargando lineups" className="flex flex-col gap-3">
      <div className="flex flex-wrap gap-2">
        <Skeleton className="h-8 w-44" />
        <Skeleton className="h-8 w-96" />
        <Skeleton className="h-8 w-36" />
      </div>
      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <Skeleton className="aspect-square w-full" />
        <div className="flex flex-col gap-2">
          <Skeleton className="h-6 w-32" />
          <Skeleton className="h-11 w-full" />
          {Array.from({ length: 7 }, (_, index) => (
            <Skeleton key={index} className="h-14 w-full" />
          ))}
        </div>
      </div>
    </div>
  );
}

/** A map's pro lineups: pick where a grenade should land on the radar, then how pros throw it there. */
export function LineupExplorer({ map }: { map: string }): ReactNode {
  const [attempt, setAttempt] = useState(0);
  const load = useProLineupMap(map, attempt);
  const header = (
    <header className="flex flex-col gap-1.5">
      <SlashTitle as="h1">{`Lineups pro de ${prettyMapName(map)}`}</SlashTitle>
      <div className="text-body-sm text-fg-2">
        <Summary load={load} />
      </div>
    </header>
  );
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

/**
 * The explorer state lives in the URL, replaced in place: the way back from a
 * lineup page lands on the same view, while going up a level is the panel's own back.
 */
function useExplorerQuery(): [ExplorerQuery, (patch: Partial<ExplorerQuery>) => void] {
  const router = useRouter();
  const pathname = usePathname();
  const search = useSearchParams().toString();
  const query = useMemo(() => readExplorerQuery(search), [search]);
  const update = useCallback(
    (patch: Partial<ExplorerQuery>) => {
      const next = writeExplorerQuery({ ...readExplorerQuery(search), ...patch });
      router.replace(next === '' ? pathname : `${pathname}?${next}`, { scroll: false });
    },
    [router, pathname, search],
  );
  return [query, update];
}

function Explorer({ data }: { data: ProLineupMap }): ReactNode {
  const [query, update] = useExplorerQuery();
  // The box answers every key at once; the URL follows it.
  const [text, setText] = useState(query.text);
  const [hover, setHover] = useState<string | null>(null);
  const geometry = data.geometry;
  const multiLevel = geometry !== undefined && isMultiLevelMap(geometry.calibration);
  const levelOf = useCallback(
    (z: number) => (geometry === undefined || !multiLevel ? RADAR_LEVELS.default : levelForAltitude(geometry.calibration, z)),
    [geometry, multiLevel],
  );
  const filters = useMemo(() => ({ ...query, text }), [query, text]);

  const allSpots = useMemo(() => landingSpots(data.lineups), [data.lineups]);
  const kept = useMemo(() => explorerLineups(data.lineups, filters), [data.lineups, filters]);
  const spots = useMemo(() => visibleSpots(allSpots, new Set(kept.map((l) => l.id))), [allSpots, kept]);
  const openSpot = query.spot === '' ? undefined : spotOf(spots, query.spot);
  const openLineup = openSpot?.lineups.find((l) => l.id === query.lineup);
  const level = openSpot === undefined ? query.level : levelOf(openSpot.landing[2]);
  const shownSpots = useMemo(() => spots.filter((s) => levelOf(s.landing[2]) === level), [spots, levelOf, level]);
  const counts = useMemo(() => {
    const out = new Map<ExplorerKind, number>();
    for (const kind of KIND_ORDER) out.set(kind, explorerLineups(data.lineups, filters, kind).length);
    return out;
  }, [data.lineups, filters]);

  const byId = useMemo(() => new Map(data.lineups.map((l) => [l.id, l])), [data.lineups]);
  const lineupsOf = useCallback((combo: ProCombo) => combo.lineups.flatMap((id) => byId.get(id) ?? []), [byId]);
  const combos = useMemo(
    () => data.combos.filter((c) => (query.side === '' || c.side === query.side) && (!query.video || Boolean(c.clip))),
    [data.combos, query.side, query.video],
  );
  const openCombo = query.view === 'combos' && query.combo !== '' ? combos.find((c) => c.id === query.combo) : undefined;
  const [metaPeriod, setMetaPeriod] = useState<MetaPeriod>('recent');
  // The meta ranks one side; with both chosen it reads T.
  const metaSide: TacticalSide = query.side === '' ? 'T' : query.side;
  const meta = useMemo(
    () =>
      query.view === 'meta'
        ? grenadeMeta(data, { side: metaSide, kinds: new Set<ProLineupKind>(query.kind === 'all' ? PRO_LINEUP_KINDS : [query.kind]), period: metaPeriod })
        : [],
    [data, query.view, query.kind, metaSide, metaPeriod],
  );

  const onText = (value: string): void => {
    setText(value);
    update({ text: value, spot: '', lineup: '' });
  };
  const clearFilters = (): void => {
    setText('');
    update({ side: '', insta: false, video: false, text: '', spot: '', lineup: '' });
  };
  const anyFilter = query.side !== '' || query.insta || query.video || text !== '' || query.kind !== DEFAULT_QUERY.kind;

  const back = useCallback((): void => {
    if (query.view === 'combos') update({ combo: '' });
    else if (query.lineup !== '') update({ lineup: '' });
    else update({ spot: '' });
  }, [query.view, query.lineup, update]);
  const step = useCallback(
    (direction: -1 | 1) => {
      if (openSpot === undefined || openLineup === undefined) return;
      const index = openSpot.lineups.indexOf(openLineup) + direction;
      const target = openSpot.lineups[index];
      if (target !== undefined) update({ lineup: target.id });
    },
    [openSpot, openLineup, update],
  );
  useEffect(() => {
    const onKey = (event: KeyboardEvent): void => {
      const target = event.target;
      if (target instanceof HTMLElement && target.closest('input, textarea, select, video, [contenteditable]')) return;
      if (event.key === 'Escape' && (openSpot !== undefined || openCombo !== undefined)) back();
      if (event.key === 'ArrowLeft') step(-1);
      if (event.key === 'ArrowRight') step(1);
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [back, step, openSpot, openCombo]);

  const openSpotById = (id: string): void => {
    const spot = spots.find((s) => s.id === id);
    if (spot !== undefined) update({ spot: spot.id, lineup: spot.lineups.length === 1 ? spot.lineups[0].id : '' });
  };
  const openLineupInExplorer = (lineup: ProLineup): void =>
    update({ view: 'lineups', kind: query.kind === 'all' ? 'all' : lineup.kind, spot: lineup.id, lineup: lineup.id, combo: '' });

  const radar = radarMarks({ query, shownSpots, openSpot, openLineup, hover, openCombo, combos, lineupsOf, meta });
  let panel: ReactNode;
  if (query.view === 'meta') {
    panel = (
      <MetaList
        rows={meta}
        period={metaPeriod}
        recentDays={data.recent_days}
        sideRounds={(metaSide === 'CT' ? data.rounds_ct : data.rounds_t)[metaPeriod]}
        onPeriod={setMetaPeriod}
        hover={hover}
        onHover={setHover}
        onOpen={(row) => openLineupInExplorer(row.lineup)}
      />
    );
  } else if (query.view === 'combos' && openCombo === undefined) {
    panel = <ComboList combos={combos} lineupsOf={lineupsOf} video={query.video} hover={hover} onHover={setHover} onOpen={(combo) => update({ combo: combo.id })} />;
  } else if (query.view === 'combos' && openCombo !== undefined) {
    panel = (
      <ComboOpen
        key={openCombo.id}
        map={data.map}
        combo={openCombo}
        lineups={lineupsOf(openCombo)}
        hover={hover}
        onHover={setHover}
        onLineup={openLineupInExplorer}
        onBack={back}
      />
    );
  } else if (openSpot === undefined) {
    panel = (
      <SpotList
        spots={shownSpots}
        mixedKinds={query.kind === 'all'}
        text={text}
        onText={onText}
        hover={hover}
        onHover={setHover}
        onOpen={(spot) => openSpotById(spot.id)}
        onClear={anyFilter ? clearFilters : null}
        video={query.video}
      />
    );
  } else if (openLineup === undefined) {
    panel = <SpotLineups spot={openSpot} video={query.video} hover={hover} onHover={setHover} onOpen={(lineup) => update({ lineup: lineup.id })} onBack={back} />;
  } else {
    panel = <LineupPreview key={openLineup.id} map={data.map} spot={openSpot} lineup={openLineup} onBack={back} onStep={(lineup) => update({ lineup: lineup.id })} />;
  }

  return (
    <div className="flex flex-col gap-3">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          label="Vista"
          value={query.view}
          options={[
            { value: 'lineups', label: 'Lineups' },
            { value: 'combos', label: `Combos (${data.combos.length})` },
            { value: 'meta', label: 'Meta' },
          ]}
          onChange={(view) => update({ view, combo: '' })}
        />
        {query.view === 'lineups' || query.view === 'meta' ? (
          <Segmented
            label="Granada"
            value={query.kind}
            options={KIND_ORDER.map((kind) => ({
              value: kind,
              label: (
                <span className="inline-flex items-center gap-1.5">
                  {kind === 'all' ? null : (
                    <span aria-hidden className="flex" style={{ color: `var(${GRENADE_KIND_TOKEN[kind]})` }}>
                      <EquipmentIcon name={grenadeIcon(kind)} height={14} />
                    </span>
                  )}
                  {KIND_TAB[kind]}
                  <span className="text-meta text-fg-3 tabular-nums">{counts.get(kind) ?? 0}</span>
                </span>
              ),
            }))}
            onChange={(kind) => update({ kind, spot: '', lineup: '' })}
          />
        ) : null}
        {query.view === 'meta' ? (
          <Segmented label="Lado" value={metaSide} options={META_SIDES} onChange={(side) => update({ side })} />
        ) : (
          <Segmented label="Lado" value={query.side} options={SIDES} onChange={(side) => update({ side, spot: '', lineup: '', combo: '' })} />
        )}
        {query.view === 'lineups' ? (
          <Toggle variant="filter" size="sm" pressed={query.insta} onPressedChange={(insta) => update({ insta, spot: '', lineup: '' })}>
            Solo instas
          </Toggle>
        ) : null}
        {query.view === 'meta' ? null : (
          <Toggle variant="filter" size="sm" pressed={query.video} onPressedChange={(video) => update({ video, spot: '', lineup: '', combo: '' })}>
            <Play aria-hidden className="size-3.5" />
            Con vídeo
          </Toggle>
        )}
        {multiLevel ? (
          <Segmented
            label="Piso"
            value={level}
            options={[RADAR_LEVELS.default, RADAR_LEVELS.lower].map((value) => ({ value, label: LEVEL_LABEL[value] }))}
            onChange={(next) => update({ level: next, spot: '', lineup: '' })}
          />
        ) : null}
      </div>

      <div className="grid items-start gap-4 lg:grid-cols-[minmax(0,1fr)_24rem]">
        <section aria-label="Radar" className="flex flex-col gap-2">
          {geometry === undefined ? (
            <p className="rounded-lg border border-border bg-surface-1 px-3 py-3 text-body-sm text-fg-2">
              Este mapa aún no trae radar; la lista tiene todos los lineups.
            </p>
          ) : (
            <ProLineupRadar
              geometry={geometry}
              level={level}
              spots={radar.spots}
              throws={radar.throws}
              onSpot={openSpotById}
              onThrow={(id) => {
                const lineup = byId.get(id);
                if (lineup === undefined) return;
                if (query.view === 'combos' || query.view === 'meta') openLineupInExplorer(lineup);
                else update({ lineup: id });
              }}
              onHoverThrow={setHover}
              label={radar.label}
            />
          )}
          <p className="text-meta text-fg-3">{radar.hint}</p>
        </section>

        <aside aria-label={{ lineups: 'Lineups', combos: 'Combos', meta: 'Meta' }[query.view]} className={PANEL}>
          {panel}
        </aside>
      </div>
    </div>
  );
}

type Marks = { spots: RadarSpot[]; throws: RadarThrow[]; label: string; hint: string };

function throwOf(lineup: ProLineup, state: RadarThrow['state'], number?: number): RadarThrow {
  return {
    id: lineup.id,
    kind: lineup.kind,
    side: lineup.side,
    from: [lineup.origin[0], lineup.origin[1]],
    to: [lineup.landing[0], lineup.landing[1]],
    label: `${PRO_KIND_LABEL[lineup.kind]} desde ${lineup.throw_place || 'zona sin nombre'}, ${techniqueLabel(lineup).toLowerCase()}`,
    number,
    state,
  };
}

function spotState(id: string, open: string | null, hover: string | null): RadarSpot['state'] {
  if (open !== null) return id === open ? 'active' : 'dim';
  return id === hover ? 'active' : 'normal';
}

function throwState(id: string, focus: string | null): RadarThrow['state'] {
  if (focus === null) return 'normal';
  return id === focus ? 'strong' : 'faint';
}

/** What the radar draws for the explorer's state: spots to pick, the open spot's throws, or a combo in order. */
function radarMarks({
  query,
  shownSpots,
  openSpot,
  openLineup,
  hover,
  openCombo,
  combos,
  lineupsOf,
  meta,
}: {
  query: ExplorerQuery;
  shownSpots: readonly ProSpot[];
  openSpot: ProSpot | undefined;
  openLineup: ProLineup | undefined;
  hover: string | null;
  openCombo: ProCombo | undefined;
  combos: readonly ProCombo[];
  lineupsOf: (combo: ProCombo) => ProLineup[];
  meta: readonly MetaRow[];
}): Marks {
  if (query.view === 'meta') {
    return {
      spots: [],
      throws: meta.slice(0, META_ON_RADAR).map((row) => throwOf(row.lineup, throwState(row.lineup.id, hover), row.rank)),
      label: 'Radar con las granadas más tiradas del lado',
      hint: `Las ${META_ON_RADAR} granadas más tiradas del lado, con su puesto en el meta. Pulsa una para abrirla.`,
    };
  }
  if (query.view === 'combos') {
    const combo = openCombo ?? combos.find((c) => c.id === hover);
    if (combo === undefined) {
      return { spots: [], throws: [], label: 'Radar del mapa', hint: 'Elige un combo para ver en el mapa desde dónde se tira cada granada y dónde cae.' };
    }
    const lineups = lineupsOf(combo);
    return {
      spots: [],
      throws: lineups.map((l, index) => throwOf(l, openCombo !== undefined && hover === l.id ? 'strong' : 'normal', index + 1)),
      label: `Radar del combo ${combo.title}`,
      hint: 'Cada número es una granada del combo, en el orden en que se tira. Pulsa uno para abrir su lineup.',
    };
  }
  const top = shownSpots.reduce((max, s) => Math.max(max, s.throws), 1);
  const spots: RadarSpot[] = shownSpots.map((s) => ({
    id: s.id,
    kind: s.kind,
    at: [s.landing[0], s.landing[1]],
    lineups: s.lineups.length,
    weight: s.throws / top,
    label: spotLabel(s),
    state: spotState(s.id, openSpot?.id ?? null, hover),
  }));
  if (openSpot === undefined) {
    return {
      spots,
      throws: [],
      label: 'Radar con los destinos de los lineups; pulsa uno para ver desde dónde se tira',
      hint: 'Cada icono es un sitio donde caen granadas pro; el número, cuántas formas hay de tirarla. Pulsa uno para ver desde dónde se lanza.',
    };
  }
  const focus = openLineup?.id ?? hover;
  return {
    spots,
    throws: openSpot.lineups.map((l, index) => throwOf(l, throwState(l.id, focus), index + 1)),
    label: `Radar: lineups de ${spotLabel(openSpot)}`,
    hint:
      openLineup === undefined
        ? 'Cada número es un sitio desde el que los pros la lanzan, el mismo que en la lista, de color T o CT. Pulsa uno para verlo.'
        : `${plainTitle(openLineup.title)}. Flechas izquierda y derecha para pasar de lineup; Esc para volver.`,
  };
}

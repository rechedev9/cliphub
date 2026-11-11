'use client';

import { useCallback, useDeferredValue, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Flame, Pause, Play, Settings2 } from 'lucide-react';
import { Button } from '@/components/ui/button';
import { InfoTip } from '@/components/analysis/info-tip';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { getSpatialHeatmap, getSpatialOverlay } from '@/lib/api/library';
import type { SpatialScope } from '@/lib/api/library';
import { HEATMAP_GRENADES, HEATMAP_KINDS, KILL_SPOTS } from '@/lib/api/spatial';
import type { HeatmapCell, HeatmapGrenade, HeatmapKind, Overlay, SpatialMatchRef, SpatialResponse } from '@/lib/api/spatial';
import { RADAR_LEVELS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarLevel, TacticalSide } from '@/lib/api/tactical';
import { eventKindLabel, placeLabel } from '@/lib/tactical-labels';
import {
  HEATMAP_KIND_LABEL,
  KILL_SPOT_LABEL,
  LEVEL_LABEL,
  arrivalStrength,
  emptyMessage,
  heatAmount,
  heatmapCellAt,
  heatmapLayer,
  heatmapLevels,
  heatmapSide,
  mergeHeatmapLayers,
  nearestCallout,
  overlayDuration,
} from '@/lib/tactical-spatial';
import type { HeatmapLayer } from '@/lib/tactical-spatial';
import { heatPlaces } from '@/lib/heat-places';
import type { HeatPlace } from '@/lib/heat-places';
import { SMOOTHING_DEFAULT, SMOOTHING_MAX, SMOOTHING_MIN, densityPicture, heatBandwidth, heatDensity } from '@/lib/heat-density';
import type { DensityPicture, HeatDensity } from '@/lib/heat-density';
import { liveHeat } from '@/lib/heat-live';
import { worldRectToRadarRect } from '@/lib/tactical-transform';
import type { WorldMark } from '@/lib/tactical-transform';
import { DEFAULT_HEAT } from '@/lib/replay/heat-choice';
import type { HeatChoice, HeatFloor } from '@/lib/replay/heat-choice';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptySample } from '@/components/analysis/empty-sample';
import { cn } from '@/lib/utils';
import { RadarSurface } from './radar-surface';
import type { RadarFrame, RadarSurfaceHandle } from './radar-surface';
import { Segmented } from '@/components/ui/segmented';
import { EvidenceChips } from './spatial-controls';
import { HeatLegend } from './heat-legend';
import { TramoBar } from '@/components/replay/tramo-bar';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { isWholeRound } from '@/lib/replay/tramo';
import { SpeedPicker } from '@/components/replay/time-strip';
import { paintHeatPlayers } from './heat-players';
import { playersSecond, useHeatPlayback } from './use-heat-playback';
import type { HeatPlayback } from './use-heat-playback';

const TOP_PLACES = 8;

/** `ready` keeps the scope its answer belongs to: a new window or kind keeps the old picture up meanwhile, a new scope does not. */
type Load = { kind: 'loading' } | { kind: 'ready'; data: SpatialResponse; scopeKey: string } | { kind: 'failed' };

/** Waits this long after the last change before asking again, so dragging the tramo asks once. */
const RUN_DELAY_MS = 180;

/** Every round's track up to the server's cap (`tacticalplan.OverlayMaxRounds`), so the map redraws a tramo without asking. */
const HEAT_TRACK_ROUNDS = 400;

/** The scope's tracks, kept with the scope they belong to. */
type Tracks = { overlay: Overlay; scopeKey: string };

/** The owner's heat choice, kept in its URL; without it the view keeps its own. */
export type HeatControl = { choice: HeatChoice; onChange: (next: HeatChoice) => void };

/** One floor's density contours, already on the scale both floors share. */
type FloorDensity = { level: RadarLevel; picture: DensityPicture };

function useHeatChoice(control: HeatControl | undefined): HeatControl {
  const [choice, setChoice] = useState<HeatChoice>(DEFAULT_HEAT);
  return control ?? { choice, onChange: setChoice };
}

/**
 * Heatmaps of the scope's rounds: where the followed players stood, died,
 * killed, landed their grenades, and how soon they first reached each spot,
 * painted as a smooth density. One side at a time, one floor or both; every
 * zone links to the rounds it counted.
 */
export function HeatmapView({
  scope,
  side: sideHint,
  onLoaded,
  tramo: tramoControl,
  heat: heatControl,
}: {
  scope: SpatialScope;
  side: TacticalSide | '';
  /** The owner's tramo; without it the view keeps its own. */
  tramo?: TramoControl;
  heat?: HeatControl;
  onLoaded?: (response: SpatialResponse) => void;
}): ReactNode {
  const { choice, onChange: setChoice } = useHeatChoice(heatControl);
  const { kind, killAt, grenades } = choice;
  const surfaceRef = useRef<RadarSurfaceHandle | null>(null);
  const repaint = useCallback(() => surfaceRef.current?.repaint(), []);
  const scopeKey = JSON.stringify(scope);
  const [tracks, setTracks] = useState<Tracks | null>(null);
  const overlay = tracks !== null && tracks.scopeKey === scopeKey ? tracks.overlay : null;
  const playback = useHeatPlayback({ control: tramoControl, onFrame: repaint, duration: overlay === null ? 0 : overlayDuration(overlay.rounds) });
  const { tramo, setTramo, playing } = playback;
  const whole = isWholeRound(tramo);
  const tramoFrom = whole ? 0 : tramo.from;
  const tramoTo = whole ? 0 : tramo.to;
  const fromPlant = tramo.fromPlant === true;
  const [side, setSide] = useState<TacticalSide | ''>(sideHint);
  const floor: HeatFloor = choice.floor;
  const [smoothing, setSmoothing] = useState(SMOOTHING_DEFAULT);
  const [selected, setSelected] = useState<string | null>(null);
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const [attempt, setAttempt] = useState(0);
  const [busy, setBusy] = useState(false);
  const scopeRef = useRef(scope);
  const imagesRef = useRef(new WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>());
  const grenadeKey = grenades.join(',');
  const loadedScope = useRef('');
  // A newer request makes an older answer stale; playback waits for the one in flight.
  const requestSeq = useRef(0);
  const inFlight = useRef(false);
  const askedFor = useRef('');
  const playingRef = useRef(playing);
  const [answers, setAnswers] = useState(0);

  useEffect(() => {
    scopeRef.current = scope;
  });

  useEffect(() => setSide(sideHint), [sideHint]);

  useEffect(() => {
    let active = true;
    getSpatialOverlay(scopeRef.current, HEAT_TRACK_ROUNDS)
      .then((answer) => {
        if (active) setTracks(answer.overlay ? { overlay: answer.overlay, scopeKey } : null);
      })
      .catch(() => {
        // Without tracks the map still paints the server's picture, a beat behind the tramo.
        if (active) setTracks(null);
      });
    return () => {
      active = false;
    };
  }, [scopeKey, attempt]);

  // Positions and arrivals redraw from the tracks when they hold every round of the scope.
  const liveKind = overlay !== null && overlay.total <= overlay.rounds.length && (kind === 'position' || kind === 'timing');

  useEffect(() => {
    playingRef.current = playing;
  });

  useEffect(() => {
    // During playback a live map needs no answer, and a server map asks again only once the last answer lands
    // and only for a tramo it has not asked about yet.
    const ask = [scopeKey, kind, killAt, tramoFrom, tramoTo, fromPlant, grenadeKey, attempt].join('|');
    if (playing && (liveKind || inFlight.current || askedFor.current === ask)) return;
    const seq = ++requestSeq.current;
    const latest = (): boolean => seq === requestSeq.current;
    setBusy(loadedScope.current === scopeKey);
    // The first answer of a scope and playback ask at once; a drag waits for the tramo to settle.
    const delay = playing || loadedScope.current !== scopeKey ? 0 : RUN_DELAY_MS;
    const timer = window.setTimeout(() => {
      inFlight.current = true;
      askedFor.current = ask;
      getSpatialHeatmap(scopeRef.current, {
        kind,
        killAt,
        from: tramoFrom,
        to: tramoTo,
        fromPlant,
        grenades: kind === 'utility' && grenadeKey !== '' ? grenadeKey.split(',').filter(isGrenade) : [],
      })
        .then((answer) => {
          inFlight.current = false;
          if (!latest()) return;
          loadedScope.current = scopeKey;
          setBusy(false);
          setLoad({ kind: 'ready', data: answer, scopeKey });
          onLoaded?.(answer);
          if (playingRef.current) setAnswers((n) => n + 1);
        })
        .catch(() => {
          inFlight.current = false;
          if (!latest()) return;
          setBusy(false);
          setLoad({ kind: 'failed' });
        });
    }, delay);
    return () => window.clearTimeout(timer);
  }, [scopeKey, kind, killAt, tramoFrom, tramoTo, fromPlant, grenadeKey, onLoaded, attempt, playing, liveKind, answers]);

  const data = load.kind === 'ready' && load.scopeKey === scopeKey ? load.data : null;
  const heatmap = data?.heatmap;
  const geometry = data?.geometry ?? null;
  const levels = useMemo(() => (heatmap ? heatmapLevels(heatmap) : []), [heatmap]);
  const twoFloors = levels.length > 1;
  const shownSide = heatmap ? heatmapSide(heatmap, side) : TACTICAL_SIDES.ct;
  const shownFloor: HeatFloor = twoFloors && (floor === 'both' || levels.includes(floor)) ? floor : (levels[0] ?? RADAR_LEVELS.default);
  const shownLevels = useMemo<RadarLevel[]>(() => (shownFloor === 'both' ? levels : [shownFloor]), [levels, shownFloor]);
  const layers = useMemo(
    () => (heatmap ? shownLevels.map((level) => ({ level, layer: heatmapLayer(heatmap, shownSide, level) })) : []),
    [heatmap, shownSide, shownLevels],
  );
  // Both floors read on one scale, so a colour means the same on either plate.
  const layer = useMemo(() => (layers.length === 0 ? null : mergeHeatmapLayers(layers.map((entry) => entry.layer))), [layers]);
  const matches = useMemo(() => new Map<string, SpatialMatchRef>((data?.matches ?? []).map((m) => [m.match_id, m])), [data]);
  const bothFloors = shownLevels.length > 1;
  // The list names places, not 64 unit cells: a place CS2 names by many cells is one line.
  const placeOf = useCallback(
    (cell: HeatmapCell): string => {
      if (!heatmap || !geometry) return '';
      const cellSize = heatmap.cell_size;
      return placeLabel(nearestCallout(geometry, cell.level, (cell.x + 0.5) * cellSize, (cell.y + 0.5) * cellSize, cell.z)) || `Celda ${cell.x}, ${cell.y}`;
    },
    [geometry, heatmap],
  );
  // The floor suffix is only shown, never part of the key, so a selection survives a floor switch.
  const places = useMemo(
    () =>
      layer === null
        ? []
        : heatPlaces({ cells: layer.cells, nameOf: placeOf }).map((place) =>
            bothFloors && place.level === RADAR_LEVELS.lower ? { ...place, name: `${place.name} (piso inferior)` } : place,
          ),
    [bothFloors, layer, placeOf],
  );
  const selectedPlace = places.find((place) => place.key === selected);

  // A drag can outrun the heat: React drops the tramos it has no time to paint, the players never lag.
  const heatTramo = useDeferredValue(tramo);
  const live = useMemo(() => {
    if (!liveKind || overlay === null || !heatmap || !geometry) return null;
    if (heatmap.kind !== 'position' && heatmap.kind !== 'timing') return null;
    return liveHeat({
      rounds: overlay.rounds,
      step: overlay.step_seconds,
      side: shownSide,
      kind: heatmap.kind,
      tramo: heatTramo,
      calibration: geometry.calibration,
      grid: heatmap.density_grid,
      cellSize: heatmap.cell_size,
    });
  }, [liveKind, overlay, heatmap, geometry, shownSide, heatTramo]);

  const densities = useMemo<FloorDensity[]>(() => {
    if (!heatmap || !geometry || !layer) return [];
    const bandwidth = heatBandwidth(geometry.calibration.map, smoothing);
    if (live !== null) {
      const grids = shownLevels.flatMap((level) => {
        const source = live.layers.find((entry) => entry.level === level);
        if (source === undefined) return [];
        const tone = source.seconds?.map((seconds) => arrivalStrength(seconds, live.maxSeconds));
        const density = heatDensity({ grid: heatmap.density_grid, bins: source.bins, bandwidth, tone });
        return density === null ? [] : [{ level, density }];
      });
      return floorPictures(grids);
    }
    const grids = shownLevels.flatMap((level) => {
      const source = heatmap.density.find((entry) => entry.side === shownSide && entry.level === level);
      if (source === undefined) return [];
      // Timing colours each spot by how early its players got there, on the legend's scale.
      const tone = heatmap.kind === 'timing' ? source.bins.map((_, i) => arrivalStrength(source.seconds?.[i] ?? 0, layer.maxSeconds)) : undefined;
      const density = heatDensity({ grid: heatmap.density_grid, bins: source.bins, bandwidth, tone });
      return density === null ? [] : [{ level, density }];
    });
    return floorPictures(grids);
  }, [heatmap, geometry, layer, live, shownLevels, shownSide, smoothing]);

  const nowRef = playback.nowRef;
  const paint = useCallback(
    ({ ctx, size, style, level }: RadarFrame) => {
      if (!heatmap || !geometry) return;
      const calibration = geometry.calibration;
      const density = densities.find((entry) => entry.level === level);
      const image = density === undefined ? null : densityImage(imagesRef.current, density.picture);
      if (density !== undefined && image !== null && heatmap.density_grid > 0) {
        // The density grid covers the whole radar square, so a square is size / grid pixels.
        const unit = size / heatmap.density_grid;
        const { x, y, width, height } = density.picture;
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(image, x * unit, y * unit, width * unit, height * unit);
      }
      if (selectedPlace !== undefined) {
        const cs = heatmap.cell_size;
        ctx.strokeStyle = style.text;
        ctx.lineWidth = Math.max(1.5, size * 0.003);
        for (const cell of selectedPlace.cells) {
          if (cell.level !== level) continue;
          const rect = worldRectToRadarRect(calibration, { minX: cell.x * cs, minY: cell.y * cs, maxX: (cell.x + 1) * cs, maxY: (cell.y + 1) * cs }, size);
          ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
        }
      }
      // At rest the players stand at the tramo's end; playback moves them every frame through the ref.
      const seconds = playing ? nowRef.current : playersSecond(tramo);
      if (overlay !== null && seconds !== null) {
        paintHeatPlayers({ ctx, size, style, level, calibration, rounds: overlay.rounds, side: shownSide, seconds, step: overlay.step_seconds, split: bothFloors });
      }
    },
    [bothFloors, densities, geometry, heatmap, nowRef, overlay, playing, selectedPlace, shownSide, tramo],
  );
  // Every side and level, so switching between them keeps the framing. Each mark carries an altitude
  // on its own floor's side of the cut, so a floor's crop only grows for the cells that stand on it.
  const marks = useMemo(() => {
    const cs = heatmap?.cell_size ?? 0;
    const cut = geometry?.calibration.lower_altitude_max;
    return (heatmap?.cells ?? []).flatMap((cell): WorldMark[] => {
      const z = cut === undefined ? [] : [cell.level === RADAR_LEVELS.lower ? cut - 1 : cut + 1];
      return [[cell.x * cs, cell.y * cs, ...z], [(cell.x + 1) * cs, (cell.y + 1) * cs, ...z]];
    });
  }, [heatmap, geometry]);

  const onPoint = useCallback(
    (world: { x: number; y: number }, level: RadarLevel) => {
      if (!heatmap) return;
      const floorLayer = layers.find((entry) => entry.level === level)?.layer;
      const cell = floorLayer ? heatmapCellAt(floorLayer, heatmap.cell_size, world.x, world.y) : undefined;
      setSelected(cell ? `${cell.level}|${placeOf(cell)}` : null);
    },
    [heatmap, layers, placeOf],
  );

  const setKind = (next: HeatmapKind): void => {
    setChoice({ ...choice, kind: next });
    setSelected(null);
  };
  const toggleGrenade = (grenade: HeatmapGrenade): void =>
    setChoice({ ...choice, grenades: grenades.includes(grenade) ? grenades.filter((g) => g !== grenade) : [...grenades, grenade] });

  const otherSide = shownSide === TACTICAL_SIDES.ct ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
  const otherSideTotal = heatmap ? heatmap.sides.filter((entry) => entry.side === otherSide).reduce((sum, entry) => sum + entry.total, 0) : 0;

  const body = (): ReactNode => {
    if (load.kind === 'failed') {
      return (
        <div role="alert" className="flex flex-wrap items-center gap-3">
          <p className="text-body-sm text-destructive">No se pudo calcular el mapa de calor. Comprueba que ClipHub Studio sigue abierto.</p>
          <Button type="button" variant="outline" size="sm" onClick={() => setAttempt((n) => n + 1)}>
            Reintentar
          </Button>
        </div>
      );
    }
    if (data === null) {
      return (
        <div role="status" aria-label="Calculando el mapa de calor" className="grid gap-4 @[56rem]:grid-cols-[minmax(0,1fr)_300px]">
          <Skeleton className="aspect-square w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      );
    }
    if (!geometry || !heatmap) {
      return (
        <EmptySample>
          <p className="text-body-sm text-fg-3">No hay partidas analizadas con este filtro.</p>
        </EmptySample>
      );
    }
    if (!layer) {
      // The scope has rounds but nothing counted in them: say what the filters left, not that matches are missing.
      return (
        <EmptySample>
          <p className="text-body-sm text-fg-2">{emptyMessage({ kind: heatmap.kind, rounds: heatmap.rounds, missing: heatmap.missing_positions, matches: heatmap.matches })}</p>
        </EmptySample>
      );
    }
    return (
      // A live map is already right while the zone list waits, and playback keeps the map bright.
      <div className={cn('grid gap-4 transition-opacity @[56rem]:grid-cols-[minmax(0,1fr)_300px]', busy && live === null && !playing && 'opacity-60')} aria-busy={busy}>
        <RadarSurface
          geometry={geometry}
          level={shownLevels[0] ?? RADAR_LEVELS.default}
          levels={shownLevels}
          paint={paint}
          marks={marks}
          handleRef={surfaceRef}
          onPoint={onPoint}
          label={`Mapa de calor de ${HEATMAP_KIND_LABEL[heatmap.kind].toLowerCase()}, lado ${shownSide}. La lista de zonas lo describe en texto.`}
        />
        <HeatmapSidebar
          kind={heatmap.kind}
          layer={live === null ? layer : { ...layer, maxSeconds: live.maxSeconds }}
          stale={playing || (busy && live !== null)}
          rounds={heatmap.rounds}
          matchCount={heatmap.matches}
          missing={heatmap.missing_positions}
          cellSize={heatmap.cell_size}
          sampleSeconds={heatmap.sample_seconds}
          places={places}
          matches={matches}
          selected={selectedPlace}
          bothFloors={bothFloors}
          otherSide={otherSideTotal > 0 ? otherSide : null}
          onOtherSide={() => {
            setSide(otherSide);
            setSelected(null);
          }}
          onSelect={(place) => setSelected(place.key)}
        />
      </div>
    );
  };

  return (
    <div className="@container flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          label="Qué cuenta el mapa"
          value={kind}
          options={HEATMAP_KINDS.map((value) => ({ value, label: HEATMAP_KIND_LABEL[value] }))}
          onChange={setKind}
        />
        <Segmented
          label="Lado"
          value={shownSide}
          options={[
            { value: TACTICAL_SIDES.ct, label: 'CT' },
            { value: TACTICAL_SIDES.t, label: 'T' },
          ]}
          onChange={(next) => {
            setSide(next);
            setSelected(null);
          }}
        />
        {twoFloors ? (
          <Segmented<HeatFloor>
            label="Piso"
            value={shownFloor}
            options={[{ value: 'both', label: 'Los dos' }, ...levels.map((value) => ({ value, label: LEVEL_LABEL[value] }))]}
            onChange={(next) => {
              setChoice({ ...choice, floor: next });
              setSelected(null);
            }}
          />
        ) : null}
        {kind === 'kill' ? (
          <Segmented
            label="Qué punto de cada baja cuenta"
            value={killAt}
            options={KILL_SPOTS.map((value) => ({ value, label: KILL_SPOT_LABEL[value] }))}
            onChange={(next) => {
              setChoice({ ...choice, killAt: next });
              setSelected(null);
            }}
          />
        ) : null}
        {kind === 'utility' ? (
          <div role="group" aria-label="Granadas" title="Sin selección cuenta todas" className="flex flex-wrap items-center gap-1.5">
            {HEATMAP_GRENADES.map((grenade) => (
              <button
                key={grenade}
                type="button"
                aria-pressed={grenades.includes(grenade)}
                onClick={() => toggleGrenade(grenade)}
                className={cn(
                  'h-7 rounded-sm border px-2 text-body-sm outline-none focus-visible:ring-[3px] focus-visible:ring-ring/50',
                  grenades.includes(grenade) ? 'border-primary text-fg-1' : 'border-border text-fg-2 hover:text-fg-1',
                )}
              >
                {eventKindLabel(grenade)}
              </button>
            ))}
          </div>
        ) : null}
        <HeatSettings smoothing={smoothing} onChange={setSmoothing} />
      </div>

      {body()}
      {/* On a narrow window the speeds wrap under the strip instead of covering it. */}
      <div className="flex flex-wrap items-center justify-end gap-1.5">
        <div className="flex min-w-[min(100%,28rem)] flex-1 items-center gap-1.5">
          <HeatPlayButton playback={playback} />
          {/* Releasing a grip or a key hands the tramo to the other forms at once. */}
          <div className="min-w-0 flex-1" onPointerUp={playback.flush} onKeyUp={playback.flush}>
            <TramoBar tramo={tramo} onChange={setTramo} />
          </div>
        </div>
        <SpeedPicker speed={playback.speed} onChange={playback.setSpeed} min={0.5} />
      </div>
    </div>
  );
}

/** Each floor's picture on the larger peak of the two, so a colour means the same on either plate. */
function floorPictures(grids: readonly { level: RadarLevel; density: HeatDensity }[]): FloorDensity[] {
  const peak = Math.max(0, ...grids.map((entry) => entry.density.peak));
  return grids.flatMap(({ level, density }) => {
    const picture = densityPicture(density, peak);
    return picture === null ? [] : [{ level, picture }];
  });
}

/** Play and pause beside the tramo: playback grows the tramo's end and the players walk with it. */
function HeatPlayButton({ playback }: { playback: HeatPlayback }): ReactNode {
  const { playing, toggle, canPlay } = playback;
  const label = playing ? 'Pausar' : 'Reproducir la ronda';
  const reason = canPlay ? label : 'Desde el plant cada ronda tiene su reloj: elige otro tramo para reproducir';
  // A disabled button shows no tooltip, so the wrapper carries the reason.
  return (
    <span title={reason} className="shrink-0">
      <Button type="button" size="icon-sm" variant="outline" className="size-10" aria-label={reason} disabled={!canPlay} onClick={toggle}>
        {playing ? <Pause aria-hidden /> : <Play aria-hidden />}
      </Button>
    </span>
  );
}

/** The contours as a bitmap, made once per picture and kept while it lives. */
export function densityImage(cache: WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>, picture: DensityPicture): HTMLCanvasElement | null {
  const known = cache.get(picture.rgba);
  if (known !== undefined) return known;
  const width = picture.width * picture.scale;
  const height = picture.height * picture.scale;
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d');
  if (ctx === null) return null;
  ctx.putImageData(new ImageData(picture.rgba, width, height), 0, 0);
  cache.set(picture.rgba, canvas);
  return canvas;
}

function isGrenade(value: string): value is HeatmapGrenade {
  return HEATMAP_GRENADES.some((grenade) => grenade === value);
}

function HeatmapSidebar({
  kind,
  layer,
  stale,
  rounds,
  matchCount,
  missing,
  cellSize,
  sampleSeconds,
  places,
  matches,
  selected,
  bothFloors,
  otherSide,
  onOtherSide,
  onSelect,
}: {
  kind: HeatmapKind;
  layer: HeatmapLayer;
  /** The zone list belongs to an earlier tramo until the server answers. */
  stale: boolean;
  rounds: number;
  matchCount: number;
  missing: number;
  cellSize: number;
  sampleSeconds: number;
  places: readonly HeatPlace[];
  matches: ReadonlyMap<string, SpatialMatchRef>;
  selected: HeatPlace | undefined;
  bothFloors: boolean;
  /** The other side when it has data and this one is empty, so the empty list can offer it. */
  otherSide: TacticalSide | null;
  onOtherSide: () => void;
  onSelect: (place: HeatPlace) => void;
}): ReactNode {
  const amount = (count: number): string => heatAmount({ kind, count, sampleSeconds });
  const top = places.slice(0, TOP_PLACES);
  const timing = kind === 'timing';
  const method = [
    `Cada zona suma las celdas de ${cellSize} unidades que cuenta su lugar; el radar las suaviza.`,
    kind === 'position' ? `El tiempo es el de los jugadores vivos, medido cada ${String(sampleSeconds).replace('.', ',')} s del tramo: una ronda larga pesa más que una corta.` : '',
    kind === 'kill' ? 'Las bajas a compañeros no cuentan.' : '',
    timing ? 'Cada jugador cuenta su primera llegada a cada zona en cada ronda.' : '',
    `${layer.rounds} de ${rounds} rondas, ${matchCount} ${matchCount === 1 ? 'partida' : 'partidas'}.`,
  ]
    .filter(Boolean)
    .join(' ');
  return (
    <aside className={cn('flex min-w-0 flex-col gap-3 transition-opacity', stale && 'opacity-60')}>
      <HeatLegend kind={kind} maxSeconds={layer.maxSeconds} />
      {missing > 0 ? <p className="text-meta text-fg-3">{missing} rondas sin posiciones guardadas.</p> : null}
      {selected ? (
        <div className="flex flex-col gap-1.5 rounded-md bg-surface-2 p-2.5">
          <span className="text-body-sm font-medium text-fg-1">{selected.name}</span>
          <span className="text-meta tabular-nums text-fg-2">
            {amount(selected.count)} · {selected.cells.length > 1 ? 'al menos ' : ''}
            {selected.rounds} {selected.rounds === 1 ? 'ronda' : 'rondas'}
            {timing ? ` · mediana ${selected.seconds} s` : ''}
          </span>
          <EvidenceChips examples={selected.examples} matches={matches} />
        </div>
      ) : null}
      {top.length === 0 ? (
        <div className="flex flex-col items-start gap-2">
          <p className="flex items-center gap-2 text-body-sm text-fg-3">
            <Flame className="size-4" aria-hidden />
            {bothFloors ? 'Nada que contar en este lado.' : 'Nada que contar en este lado y piso.'}
          </p>
          {otherSide === null ? null : (
            <Button type="button" variant="outline" size="xs" onClick={onOtherSide}>
              Ver el lado {otherSide}
            </Button>
          )}
        </div>
      ) : (
        <table className="w-full text-body-sm">
          <caption className="pb-1 text-left text-meta text-fg-3">
            <span className="flex items-center justify-between gap-2">
              {timing ? 'Primeras llegadas' : 'Zonas con más actividad'}
              <InfoTip label="el mapa de calor" text={method} />
            </span>
          </caption>
          <thead className="sr-only">
            <tr>
              <th>Zona</th>
              <th>Cantidad</th>
              {timing ? <th>Mediana</th> : null}
            </tr>
          </thead>
          <tbody>
            {top.map((place) => (
              <tr key={place.key} className={cn('border-t border-border-subtle', selected?.key === place.key && 'bg-surface-3')}>
                <td className="py-1 pr-2">
                  <button type="button" onClick={() => onSelect(place)} className="text-left text-fg-1 hover:underline">
                    {place.name}
                  </button>
                </td>
                <td className="py-1 text-right tabular-nums text-fg-2">{amount(place.count)}</td>
                {timing ? <td className="py-1 pl-2 text-right tabular-nums text-fg-3">{place.seconds} s</td> : null}
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </aside>
  );
}

/** Secondary options of the heatmap in one menu, so the filter row carries only what changes the question. */
function HeatSettings({ smoothing, onChange }: { smoothing: number; onChange: (smoothing: number) => void }): ReactNode {
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button type="button" variant="ghost" size="icon-sm" aria-label="Ajustes del mapa de calor" className="ml-auto">
          <Settings2 aria-hidden />
        </Button>
      </PopoverTrigger>
      <PopoverContent align="end" className="w-64">
        <label className="flex items-center gap-2 text-body-sm text-fg-2">
          Suavizado
          <input
            type="range"
            min={Math.round(SMOOTHING_MIN * 100)}
            max={Math.round(SMOOTHING_MAX * 100)}
            step={1}
            value={Math.round(smoothing * 100)}
            onChange={(event) => onChange(Number(event.target.value) / 100)}
            className="h-1 w-full min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-primary"
          />
          <span className="w-10 text-right text-meta tabular-nums text-fg-3">{Math.round(smoothing * 100)} %</span>
        </label>
      </PopoverContent>
    </Popover>
  );
}

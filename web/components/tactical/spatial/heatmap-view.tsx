'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Flame } from 'lucide-react';
import { getSpatialHeatmap } from '@/lib/api/library';
import type { SpatialScope } from '@/lib/api/library';
import { HEATMAP_GRENADES, HEATMAP_KINDS, KILL_SPOTS } from '@/lib/api/spatial';
import type { HeatmapCell, HeatmapGrenade, HeatmapKind, SpatialMatchRef, SpatialResponse } from '@/lib/api/spatial';
import { RADAR_LEVELS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarLevel, TacticalGeometry, TacticalSide } from '@/lib/api/tactical';
import { eventKindLabel, placeLabel } from '@/lib/tactical-labels';
import {
  HEATMAP_KIND_LABEL,
  HEATMAP_LEGEND_LABEL,
  HEATMAP_UNIT_LABEL,
  KILL_SPOT_LABEL,
  LEVEL_LABEL,
  HEAT_GRADIENT,
  arrivalStrength,
  heatmapCellAt,
  heatmapLayer,
  heatmapLevels,
  heatmapSide,
  mergeHeatmapLayers,
  nearestCallout,
  topCells,
} from '@/lib/tactical-spatial';
import type { HeatmapLayer } from '@/lib/tactical-spatial';
import { SMOOTHING_DEFAULT, SMOOTHING_MAX, SMOOTHING_MIN, densityPicture, heatBandwidth, heatDensity } from '@/lib/heat-density';
import type { DensityPicture } from '@/lib/heat-density';
import { worldRectToRadarRect } from '@/lib/tactical-transform';
import type { WorldMark } from '@/lib/tactical-transform';
import { DEFAULT_HEAT } from '@/lib/replay/heat-choice';
import type { HeatChoice } from '@/lib/replay/heat-choice';
import { Skeleton } from '@/components/ui/skeleton';
import { EmptySample } from '@/components/analysis/empty-sample';
import { cn } from '@/lib/utils';
import { RadarSurface } from './radar-surface';
import type { RadarFrame } from './radar-surface';
import { Segmented } from '@/components/ui/segmented';
import { EvidenceChips } from './spatial-controls';
import { TramoBar, useTramo } from '@/components/replay/tramo-bar';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { isWholeRound } from '@/lib/replay/tramo';

const TOP_CELLS = 8;

type Load = { kind: 'loading' } | { kind: 'ready'; data: SpatialResponse } | { kind: 'failed' };

/** One floor or both side by side, on a two-floor map. */
type FloorChoice = RadarLevel | 'both';

/** The owner's heat choice, kept in its URL; without it the view keeps its own. */
export type HeatControl = { choice: HeatChoice; onChange: (next: HeatChoice) => void };

/** One floor's density contours, already on the scale both floors share. */
type FloorDensity = { level: RadarLevel; picture: DensityPicture };

function cellKey(cell: HeatmapCell): string {
  return `${cell.side}|${cell.level}|${cell.x}|${cell.y}`;
}

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
  const { tramo, onChange: setTramo } = useTramo(tramoControl);
  const whole = isWholeRound(tramo);
  const tramoFrom = whole ? 0 : tramo.from;
  const tramoTo = whole ? 0 : tramo.to;
  const fromPlant = tramo.fromPlant === true;
  const [side, setSide] = useState<TacticalSide | ''>(sideHint);
  const [floor, setFloor] = useState<FloorChoice>('both');
  const [smoothing, setSmoothing] = useState(SMOOTHING_DEFAULT);
  const [selected, setSelected] = useState<string | null>(null);
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const scopeKey = JSON.stringify(scope);
  const scopeRef = useRef(scope);
  const imagesRef = useRef(new WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>());
  const grenadeKey = grenades.join(',');

  useEffect(() => {
    scopeRef.current = scope;
  });

  useEffect(() => setSide(sideHint), [sideHint]);

  useEffect(() => {
    let active = true;
    setLoad({ kind: 'loading' });
    getSpatialHeatmap(scopeRef.current, {
      kind,
      killAt,
      from: tramoFrom,
      to: tramoTo,
      fromPlant,
      grenades: kind === 'utility' && grenadeKey !== '' ? grenadeKey.split(',').filter(isGrenade) : [],
    })
      .then((data) => {
        if (!active) return;
        setLoad({ kind: 'ready', data });
        onLoaded?.(data);
      })
      .catch(() => {
        if (active) setLoad({ kind: 'failed' });
      });
    return () => {
      active = false;
    };
  }, [scopeKey, kind, killAt, tramoFrom, tramoTo, fromPlant, grenadeKey, onLoaded]);

  const data = load.kind === 'ready' ? load.data : null;
  const heatmap = data?.heatmap;
  const geometry = data?.geometry ?? null;
  const levels = useMemo(() => (heatmap ? heatmapLevels(heatmap) : []), [heatmap]);
  const twoFloors = levels.length > 1;
  const shownSide = heatmap ? heatmapSide(heatmap, side) : TACTICAL_SIDES.ct;
  const shownFloor: FloorChoice = twoFloors && (floor === 'both' || levels.includes(floor)) ? floor : (levels[0] ?? RADAR_LEVELS.default);
  const shownLevels = useMemo<RadarLevel[]>(() => (shownFloor === 'both' ? levels : [shownFloor]), [levels, shownFloor]);
  const layers = useMemo(
    () => (heatmap ? shownLevels.map((level) => ({ level, layer: heatmapLayer(heatmap, shownSide, level) })) : []),
    [heatmap, shownSide, shownLevels],
  );
  // Both floors read on one scale, so a colour means the same on either plate.
  const layer = useMemo(() => (layers.length === 0 ? null : mergeHeatmapLayers(layers.map((entry) => entry.layer))), [layers]);
  const matches = useMemo(() => new Map<string, SpatialMatchRef>((data?.matches ?? []).map((m) => [m.match_id, m])), [data]);
  const selectedCell = layer?.cells.find((cell) => cellKey(cell) === selected);

  const densities = useMemo<FloorDensity[]>(() => {
    if (!heatmap || !geometry || !layer) return [];
    const bandwidth = heatBandwidth(geometry.calibration.map, smoothing);
    const grids = shownLevels.flatMap((level) => {
      const source = heatmap.density.find((entry) => entry.side === shownSide && entry.level === level);
      if (source === undefined) return [];
      // Timing colours each spot by how early its players got there, on the legend's scale.
      const tone = heatmap.kind === 'timing' ? source.bins.map((_, i) => arrivalStrength(source.seconds?.[i] ?? 0, layer.maxSeconds)) : undefined;
      const density = heatDensity({ grid: heatmap.density_grid, bins: source.bins, bandwidth, tone });
      return density === null ? [] : [{ level, density }];
    });
    const peak = Math.max(0, ...grids.map((entry) => entry.density.peak));
    return grids.flatMap(({ level, density }) => {
      const picture = densityPicture(density, peak);
      return picture === null ? [] : [{ level, picture }];
    });
  }, [heatmap, geometry, layer, shownLevels, shownSide, smoothing]);

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
      if (selectedCell !== undefined && selectedCell.level === level) {
        const cs = heatmap.cell_size;
        const rect = worldRectToRadarRect(
          calibration,
          { minX: selectedCell.x * cs, minY: selectedCell.y * cs, maxX: (selectedCell.x + 1) * cs, maxY: (selectedCell.y + 1) * cs },
          size,
        );
        ctx.strokeStyle = style.text;
        ctx.lineWidth = Math.max(1.5, size * 0.003);
        ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
      }
    },
    [densities, geometry, heatmap, selectedCell],
  );
  // Every side and level, so switching between them keeps the framing.
  const marks = useMemo(() => {
    const cs = heatmap?.cell_size ?? 0;
    return (heatmap?.cells ?? []).flatMap((cell): WorldMark[] => [[cell.x * cs, cell.y * cs], [(cell.x + 1) * cs, (cell.y + 1) * cs]]);
  }, [heatmap]);

  const onPoint = useCallback(
    (world: { x: number; y: number }, level: RadarLevel) => {
      if (!heatmap) return;
      const floorLayer = layers.find((entry) => entry.level === level)?.layer;
      const cell = floorLayer ? heatmapCellAt(floorLayer, heatmap.cell_size, world.x, world.y) : undefined;
      setSelected(cell ? cellKey(cell) : null);
    },
    [heatmap, layers],
  );

  const setKind = (next: HeatmapKind): void => {
    setChoice({ ...choice, kind: next });
    setSelected(null);
  };
  const toggleGrenade = (grenade: HeatmapGrenade): void =>
    setChoice({ ...choice, grenades: grenades.includes(grenade) ? grenades.filter((g) => g !== grenade) : [...grenades, grenade] });

  const body = (): ReactNode => {
    if (load.kind === 'failed') {
      return (
        <p role="alert" className="text-body-sm text-destructive">
          No se pudo calcular el mapa de calor. Comprueba que ClipHub Studio sigue abierto.
        </p>
      );
    }
    if (load.kind === 'loading') {
      return (
        <div role="status" aria-label="Calculando el mapa de calor" className="grid gap-4 @[56rem]:grid-cols-[minmax(0,1fr)_300px]">
          <Skeleton className="aspect-square w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      );
    }
    if (!geometry || !heatmap || !layer) {
      return (
        <EmptySample>
          <p className="text-body-sm text-fg-3">No hay partidas analizadas con este filtro.</p>
        </EmptySample>
      );
    }
    return (
      <div className="grid gap-4 @[56rem]:grid-cols-[minmax(0,1fr)_300px]">
        <RadarSurface
          geometry={geometry}
          level={shownLevels[0] ?? RADAR_LEVELS.default}
          levels={shownLevels}
          paint={paint}
          marks={marks}
          onPoint={onPoint}
          label={`Mapa de calor de ${HEATMAP_KIND_LABEL[heatmap.kind].toLowerCase()}, lado ${shownSide}. La lista de zonas lo describe en texto.`}
        />
        <HeatmapSidebar
          kind={heatmap.kind}
          layer={layer}
          rounds={heatmap.rounds}
          matchCount={heatmap.matches}
          missing={heatmap.missing_positions}
          cellSize={heatmap.cell_size}
          samplesPerRound={heatmap.samples_per_round}
          geometry={geometry}
          matches={matches}
          selected={selectedCell}
          bothFloors={shownLevels.length > 1}
          onSelect={(cell) => setSelected(cellKey(cell))}
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
          <Segmented<FloorChoice>
            label="Piso"
            value={shownFloor}
            options={[{ value: 'both', label: 'Los dos' }, ...levels.map((value) => ({ value, label: LEVEL_LABEL[value] }))]}
            onChange={(next) => {
              setFloor(next);
              setSelected(null);
            }}
          />
        ) : null}
      </div>
      <TramoBar tramo={tramo} onChange={setTramo} />
      <div className="flex flex-wrap items-center gap-x-4 gap-y-2">
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
          <div role="group" aria-label="Granadas" className="flex flex-wrap items-center gap-1.5">
            <span className="text-body-sm text-fg-3">Granadas:</span>
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
            <span className="text-meta text-fg-3">{grenades.length === 0 ? 'Sin selección cuenta todas.' : ''}</span>
          </div>
        ) : null}
        <label className="flex min-w-56 items-center gap-2 text-body-sm text-fg-2">
          Suavizado
          <input
            type="range"
            min={Math.round(SMOOTHING_MIN * 100)}
            max={Math.round(SMOOTHING_MAX * 100)}
            step={1}
            value={Math.round(smoothing * 100)}
            onChange={(event) => setSmoothing(Number(event.target.value) / 100)}
            className="h-1 w-full min-w-0 flex-1 cursor-pointer appearance-none rounded-full bg-border-strong accent-primary"
          />
          <span className="w-10 text-right text-meta tabular-nums text-fg-3">{Math.round(smoothing * 100)} %</span>
        </label>
      </div>

      {body()}
    </div>
  );
}

/** The contours as a bitmap, made once per picture and kept while it lives. */
function densityImage(cache: WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>, picture: DensityPicture): HTMLCanvasElement | null {
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
  rounds,
  matchCount,
  missing,
  cellSize,
  samplesPerRound,
  geometry,
  matches,
  selected,
  bothFloors,
  onSelect,
}: {
  kind: HeatmapKind;
  layer: HeatmapLayer;
  rounds: number;
  matchCount: number;
  missing: number;
  cellSize: number;
  samplesPerRound: number;
  geometry: TacticalGeometry;
  matches: ReadonlyMap<string, SpatialMatchRef>;
  selected: HeatmapCell | undefined;
  bothFloors: boolean;
  onSelect: (cell: HeatmapCell) => void;
}): ReactNode {
  const unit = HEATMAP_UNIT_LABEL[kind];
  const zone = (cell: HeatmapCell): string => {
    const name =
      placeLabel(nearestCallout(geometry, cell.level, (cell.x + 0.5) * cellSize, (cell.y + 0.5) * cellSize, cell.z)) || `Celda ${cell.x}, ${cell.y}`;
    return bothFloors && cell.level === RADAR_LEVELS.lower ? `${name} (piso inferior)` : name;
  };
  const top = topCells(layer, TOP_CELLS);
  const timing = kind === 'timing';
  return (
    <aside className="flex min-w-0 flex-col gap-3 rounded-lg border border-border-strong bg-surface-1 p-3">
      <p className="text-body-sm text-fg-2">
        {layer.total} {unit} en {layer.rounds} de {rounds} rondas ({matchCount} {matchCount === 1 ? 'partida' : 'partidas'}).
        {missing > 0 ? ` ${missing} sin posiciones guardadas.` : ''}
      </p>
      <Legend kind={kind} maxCount={layer.maxCount} maxSeconds={layer.maxSeconds} />
      {selected ? (
        <div className="flex flex-col gap-1.5 rounded-md border border-border p-2.5">
          <span className="text-body-sm font-medium text-fg-1">{zone(selected)}</span>
          <span className="text-meta tabular-nums text-fg-2">
            {selected.count} {unit} · {selected.rounds} {selected.rounds === 1 ? 'ronda' : 'rondas'}
            {timing ? ` · mediana ${selected.seconds} s` : ''}
          </span>
          <EvidenceChips examples={selected.examples} matches={matches} />
        </div>
      ) : (
        <p className="text-meta text-fg-3">Pulsa una zona del radar para ver de qué rondas sale.</p>
      )}
      {top.length === 0 ? (
        <p className="flex items-center gap-2 text-body-sm text-fg-3">
          <Flame className="size-4" aria-hidden />
          {bothFloors ? 'Nada que contar en este lado.' : 'Nada que contar en este lado y piso.'}
        </p>
      ) : (
        <table className="w-full text-body-sm">
          <caption className="pb-1 text-left text-meta text-fg-3">{timing ? 'Primeras llegadas' : 'Zonas con más actividad'}</caption>
          <thead className="sr-only">
            <tr>
              <th>Zona</th>
              <th>Cantidad</th>
              {timing ? <th>Mediana</th> : null}
            </tr>
          </thead>
          <tbody>
            {top.map((cell) => (
              <tr key={cellKey(cell)} className={cn('border-t border-border-subtle', selected && cellKey(selected) === cellKey(cell) && 'bg-surface-3')}>
                <td className="py-1 pr-2">
                  <button type="button" onClick={() => onSelect(cell)} className="text-left text-fg-1 hover:underline">
                    {zone(cell)}
                  </button>
                </td>
                <td className="py-1 text-right tabular-nums text-fg-2">{cell.count}</td>
                {timing ? <td className="py-1 pl-2 text-right tabular-nums text-fg-3">{cell.seconds} s</td> : null}
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="text-meta text-fg-3">
        Zonas de {cellSize} unidades; el radar las suaviza.
        {kind === 'position' ? ` Cada ronda se lee en ${samplesPerRound} instantes del tramo, una muestra por jugador vivo en cada uno.` : ''}
        {kind === 'kill' ? ' Las bajas a compañeros no cuentan.' : ''}
        {timing ? ' Cada jugador cuenta su primera llegada a cada zona en cada ronda.' : ''}
      </p>
    </aside>
  );
}

/** The heat scale, dark green to yellow, with ends in what the map counts; timing runs reversed. */
function Legend({ kind, maxCount, maxSeconds }: { kind: HeatmapKind; maxCount: number; maxSeconds: number }): ReactNode {
  const timing = kind === 'timing';
  const unit = HEATMAP_UNIT_LABEL[kind];
  return (
    <div className="flex flex-col gap-1" aria-label={`Leyenda: ${HEATMAP_LEGEND_LABEL[kind]}`}>
      <span className="text-meta text-fg-2">{HEATMAP_LEGEND_LABEL[kind]}</span>
      <div className="h-2 rounded-sm" style={{ background: HEAT_GRADIENT }} aria-hidden />
      <div className="flex justify-between text-meta tabular-nums text-fg-3">
        <span>{timing ? `${maxSeconds} s, más tarde` : `0 ${unit}`}</span>
        <span>{timing ? '0 s, antes' : `${maxCount} ${unit}`}</span>
      </div>
    </div>
  );
}

'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Flame } from 'lucide-react';
import { getSpatialHeatmap } from '@/lib/api/library';
import type { SpatialScope } from '@/lib/api/library';
import { HEATMAP_GRENADES, HEATMAP_KINDS } from '@/lib/api/spatial';
import type { HeatmapCell, HeatmapGrenade, HeatmapKind, SpatialMatchRef, SpatialResponse } from '@/lib/api/spatial';
import { RADAR_LEVELS, TACTICAL_SIDES } from '@/lib/api/tactical';
import type { RadarLevel, TacticalSide } from '@/lib/api/tactical';
import { eventKindLabel } from '@/lib/tactical-labels';
import {
  HEATMAP_KIND_LABEL,
  HEATMAP_UNIT_LABEL,
  LEVEL_LABEL,
  heatStrength,
  heatmapCellAt,
  heatmapLayer,
  heatmapLevels,
  heatmapSide,
  nearestCallout,
  topCells,
} from '@/lib/tactical-spatial';
import { worldRectToRadarRect } from '@/lib/tactical-transform';
import { Skeleton } from '@/components/ui/skeleton';
import { cn } from '@/lib/utils';
import { RadarSurface } from './radar-surface';
import type { RadarFrame } from './radar-surface';
import { EvidenceChips, Segmented } from './spatial-controls';

const WINDOWS = [
  { value: 'all', label: 'Toda la ronda', from: 0, to: 0 },
  { value: 'early', label: '0 a 20 s', from: 0, to: 20 },
  { value: 'mid', label: '20 a 40 s', from: 20, to: 40 },
  { value: 'late', label: '40 a 60 s', from: 40, to: 60 },
  { value: 'after', label: 'Desde 60 s', from: 60, to: 0 },
] as const;
type WindowValue = (typeof WINDOWS)[number]['value'];

const TOP_CELLS = 8;

type Load = { kind: 'loading' } | { kind: 'ready'; data: SpatialResponse } | { kind: 'failed' };

function cellKey(cell: HeatmapCell): string {
  return `${cell.side}|${cell.level}|${cell.x}|${cell.y}`;
}

/**
 * Heatmaps of the scope's rounds: where the followed players stood, died,
 * landed their grenades, and how soon they first reached each spot. One side
 * and one level at a time; every cell links to the rounds it counted.
 */
export function HeatmapView({
  scope,
  side: sideHint,
  onLoaded,
}: {
  scope: SpatialScope;
  side: TacticalSide | '';
  onLoaded?: (response: SpatialResponse) => void;
}): ReactNode {
  const [kind, setKind] = useState<HeatmapKind>('position');
  const [windowValue, setWindowValue] = useState<WindowValue>('all');
  const [grenades, setGrenades] = useState<HeatmapGrenade[]>([]);
  const [side, setSide] = useState<TacticalSide | ''>(sideHint);
  const [level, setLevel] = useState<RadarLevel>(RADAR_LEVELS.default);
  const [selected, setSelected] = useState<string | null>(null);
  const [load, setLoad] = useState<Load>({ kind: 'loading' });
  const scopeKey = JSON.stringify(scope);
  const scopeRef = useRef(scope);
  const grenadeKey = grenades.join(',');

  useEffect(() => {
    scopeRef.current = scope;
  });

  useEffect(() => setSide(sideHint), [sideHint]);

  useEffect(() => {
    let active = true;
    setLoad({ kind: 'loading' });
    const range = WINDOWS.find((entry) => entry.value === windowValue) ?? WINDOWS[0];
    getSpatialHeatmap(scopeRef.current, {
      kind,
      from: range.from,
      to: range.to,
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
  }, [scopeKey, kind, windowValue, grenadeKey, onLoaded]);

  const data = load.kind === 'ready' ? load.data : null;
  const heatmap = data?.heatmap;
  const levels = heatmap ? heatmapLevels(heatmap) : [];
  const shownSide = heatmap ? heatmapSide(heatmap, side) : TACTICAL_SIDES.ct;
  const shownLevel = levels.includes(level) ? level : (levels[0] ?? RADAR_LEVELS.default);
  const layer = useMemo(() => (heatmap ? heatmapLayer(heatmap, shownSide, shownLevel) : null), [heatmap, shownSide, shownLevel]);
  const matches = useMemo(() => new Map<string, SpatialMatchRef>((data?.matches ?? []).map((m) => [m.match_id, m])), [data]);
  const selectedCell = layer?.cells.find((cell) => cellKey(cell) === selected);

  const paint = useCallback(
    ({ ctx, size, style }: RadarFrame) => {
      if (!heatmap || !layer || !data?.geometry) return;
      const calibration = data.geometry.calibration;
      const color = shownSide === TACTICAL_SIDES.t ? style.t : style.ct;
      const cs = heatmap.cell_size;
      for (const cell of layer.cells) {
        const rect = worldRectToRadarRect(
          calibration,
          { minX: cell.x * cs, minY: cell.y * cs, maxX: (cell.x + 1) * cs, maxY: (cell.y + 1) * cs },
          size,
        );
        ctx.globalAlpha = 0.14 + 0.76 * heatStrength(heatmap.kind, cell, layer);
        ctx.fillStyle = color;
        ctx.fillRect(rect.x, rect.y, rect.width + 0.5, rect.height + 0.5);
        if (cellKey(cell) === selected) {
          ctx.globalAlpha = 1;
          ctx.strokeStyle = style.text;
          ctx.lineWidth = Math.max(1.5, size * 0.003);
          ctx.strokeRect(rect.x, rect.y, rect.width, rect.height);
        }
      }
      ctx.globalAlpha = 1;
    },
    [data, heatmap, layer, selected, shownSide],
  );

  const onPoint = useCallback(
    (world: { x: number; y: number }) => {
      if (!heatmap || !layer) return;
      const cell = heatmapCellAt(layer, heatmap.cell_size, world.x, world.y);
      setSelected(cell ? cellKey(cell) : null);
    },
    [heatmap, layer],
  );

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
    if (!data?.geometry || !heatmap || !layer) {
      return <p className="text-body-sm text-fg-3">No hay partidas analizadas con este filtro.</p>;
    }
    return (
      <div className="grid gap-4 @[56rem]:grid-cols-[minmax(0,1fr)_300px]">
        <RadarSurface
          geometry={data.geometry}
          level={shownLevel}
          paint={paint}
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
          callouts={data.geometry.callouts}
          matches={matches}
          selected={selectedCell}
          onSelect={(cell) => setSelected(cellKey(cell))}
          side={shownSide}
        />
      </div>
    );
  };

  const toggleGrenade = (grenade: HeatmapGrenade): void =>
    setGrenades((current) => (current.includes(grenade) ? current.filter((g) => g !== grenade) : [...current, grenade]));

  return (
    <div className="@container flex flex-col gap-4">
      <div className="flex flex-wrap items-center gap-2">
        <Segmented
          label="Qué cuenta el mapa"
          value={kind}
          options={HEATMAP_KINDS.map((value) => ({ value, label: HEATMAP_KIND_LABEL[value] }))}
          onChange={(next) => {
            setKind(next);
            setSelected(null);
          }}
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
        {levels.length > 1 ? (
          <Segmented
            label="Piso"
            value={shownLevel}
            options={levels.map((value) => ({ value, label: LEVEL_LABEL[value] }))}
            onChange={(next) => {
              setLevel(next);
              setSelected(null);
            }}
          />
        ) : null}
        <Segmented label="Momento de la ronda" value={windowValue} options={WINDOWS} onChange={setWindowValue} />
      </div>
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

      {body()}
    </div>
  );
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
  callouts,
  matches,
  selected,
  onSelect,
  side,
}: {
  side: TacticalSide;
  kind: HeatmapKind;
  layer: ReturnType<typeof heatmapLayer>;
  rounds: number;
  matchCount: number;
  missing: number;
  cellSize: number;
  callouts: { name: string; level: string; center: [number, number] }[];
  matches: ReadonlyMap<string, SpatialMatchRef>;
  selected: HeatmapCell | undefined;
  onSelect: (cell: HeatmapCell) => void;
}): ReactNode {
  const unit = HEATMAP_UNIT_LABEL[kind];
  const zone = (cell: HeatmapCell): string =>
    nearestCallout(callouts, cell.level, (cell.x + 0.5) * cellSize, (cell.y + 0.5) * cellSize) || `Celda ${cell.x}, ${cell.y}`;
  const top = topCells(layer, TOP_CELLS);
  return (
    <aside className="flex min-w-0 flex-col gap-3 rounded-lg border border-border-strong bg-surface-1 p-3">
      <p className="text-body-sm text-fg-2">
        {layer.total} {unit} en {layer.rounds} de {rounds} rondas ({matchCount} {matchCount === 1 ? 'partida' : 'partidas'}).
        {missing > 0 ? ` ${missing} sin posiciones guardadas.` : ''}
      </p>
      <Legend kind={kind} side={side} maxCount={layer.maxCount} maxSeconds={layer.maxSeconds} />
      {selected ? (
        <div className="flex flex-col gap-1.5 rounded-md border border-border p-2.5">
          <span className="text-body-sm font-medium text-fg-1">{zone(selected)}</span>
          <span className="font-mono text-meta tabular-nums text-fg-2">
            {selected.count} {unit} · {selected.rounds} {selected.rounds === 1 ? 'ronda' : 'rondas'} · mediana {selected.seconds} s
          </span>
          <EvidenceChips examples={selected.examples} matches={matches} />
        </div>
      ) : (
        <p className="text-meta text-fg-3">Pulsa una zona del radar para ver de qué rondas sale.</p>
      )}
      {top.length === 0 ? (
        <p className="flex items-center gap-2 text-body-sm text-fg-3">
          <Flame className="size-4" aria-hidden />
          Nada que contar en este lado y piso.
        </p>
      ) : (
        <table className="w-full text-body-sm">
          <caption className="pb-1 text-left text-meta text-fg-3">{kind === 'timing' ? 'Primeras llegadas' : 'Zonas con más actividad'}</caption>
          <thead className="sr-only">
            <tr>
              <th>Zona</th>
              <th>Cantidad</th>
              <th>Mediana</th>
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
                <td className="py-1 text-right font-mono tabular-nums text-fg-2">{cell.count}</td>
                <td className="py-1 pl-2 text-right font-mono tabular-nums text-fg-3">{cell.seconds} s</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      <p className="text-meta text-fg-3">
        Celdas de {cellSize} unidades.
        {kind === 'position' ? ' Una muestra por jugador vivo y segundo.' : ''}
        {kind === 'timing' ? ' Cada jugador cuenta su primera llegada a cada celda en cada ronda.' : ''}
      </p>
    </aside>
  );
}

/** The radar paints CT in --primary and T in --warning; the legend follows it. */
const SIDE_RAMP: Record<TacticalSide, string> = {
  [TACTICAL_SIDES.ct]: 'var(--primary)',
  [TACTICAL_SIDES.t]: 'var(--warning)',
};

function Legend({ kind, side, maxCount, maxSeconds }: { kind: HeatmapKind; side: TacticalSide; maxCount: number; maxSeconds: number }): ReactNode {
  const timing = kind === 'timing';
  const color = SIDE_RAMP[side];
  return (
    <div className="flex flex-col gap-1" aria-label="Leyenda">
      <div
        className="h-2 rounded-sm"
        style={{ background: `linear-gradient(to right, color-mix(in oklab, ${color} 15%, transparent), ${color})` }}
        aria-hidden
      />
      <div className="flex justify-between font-mono text-meta tabular-nums text-fg-3">
        <span>{timing ? `${maxSeconds} s` : '1'}</span>
        <span>{timing ? '0 s, llegan antes' : `${maxCount} ${HEATMAP_UNIT_LABEL[kind]}`}</span>
      </div>
    </div>
  );
}

'use client';

import { useCallback, useEffect, useMemo, useRef, useState, type PointerEvent, type ReactNode } from 'react';
import type { Heatmap, Overlay } from '@/lib/api/spatial';
import { PATTERN_MAX_POLYGON_POINTS, type PatternArea } from '@/lib/api/patterns';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry, type TacticalSide } from '@/lib/api/tactical';
import { dragWorldBox, type WorldBox } from '@/lib/analysis/grenade-patterns';
import { areaOfBox, areaOfPolygon, boxOfArea, patternPlayerToken, trackInWindow } from '@/lib/analysis/pattern-finder';
import { densityPicture, heatBandwidth, heatDensity, SMOOTHING_DEFAULT } from '@/lib/heat-density';
import type { Tramo } from '@/lib/replay/tramo';
import { heatmapSide, renderedToWorld, type RadarProjection } from '@/lib/tactical-spatial';
import { levelForAltitude, worldToRendered, type RadarPoint } from '@/lib/tactical-transform';
import { cn } from '@/lib/utils';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { densityImage } from '@/components/tactical/spatial/heatmap-view';

/** A drag shorter than this many pixels is a click and clears the zone. */
const DRAG_MIN = 4;
/** A click this close to the first corner closes the polygon. */
const CLOSE_PX = 10;

/** How a drag on the radar draws the zone; `none` when the zone is a set of callouts. */
export type ZoneTool = 'box' | 'polygon' | 'none';

export type PatternPicture = 'recorridos' | 'calor';

type Props = {
  geometry: TacticalGeometry;
  levels: readonly RadarLevel[];
  /** The map has two floors: paths and zones keep to the floor they are on, even with one shown. */
  multiLevel: boolean;
  picture: PatternPicture;
  overlay: Overlay | null;
  heatmap: Heatmap | null;
  side: TacticalSide | '';
  /** The players in focus by SteamID64, in the order their colours follow; empty colours everyone. */
  focus: readonly string[];
  /** The clock window the paths are cut to. */
  tramo: Tramo;
  /** The zone of the block being edited, and the zones of the other blocks drawn fainter. */
  area: PatternArea | null;
  others: readonly PatternArea[];
  tool: ZoneTool;
  onArea: (area: PatternArea | null) => void;
};

/** The matched rounds' paths in the window (focus loud) or their heatmap; dragging or clicking draws the edited block's zone. */
export function PatternRadar({ geometry, levels, multiLevel, picture, overlay, heatmap, side, focus, tramo, area, others, tool, onArea }: Props): ReactNode {
  const split = multiLevel;
  const { calibration } = geometry;
  const images = useRef(new WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>());
  const densities = useMemo(() => {
    if (heatmap === null || picture !== 'calor') return [];
    const shown = heatmapSide(heatmap, side);
    const bandwidth = heatBandwidth(calibration.map, SMOOTHING_DEFAULT);
    const grids = levels.flatMap((level) => {
      const source = heatmap.density.find((entry) => entry.side === shown && entry.level === level);
      const density = source === undefined ? null : heatDensity({ grid: heatmap.density_grid, bins: source.bins, bandwidth });
      return density === null ? [] : [{ level, density }];
    });
    const peak = Math.max(0, ...grids.map((entry) => entry.density.peak));
    return grids.flatMap(({ level, density }) => {
      const out = densityPicture(density, peak);
      return out === null ? [] : [{ level, picture: out }];
    });
  }, [calibration.map, heatmap, levels, picture, side]);

  const paint = useCallback(
    ({ ctx, size, level }: RadarFrame) => {
      if (picture === 'calor') {
        const density = densities.find((entry) => entry.level === level);
        const image = density === undefined ? null : densityImage(images.current, density.picture);
        if (density === undefined || image === null || heatmap === null || heatmap.density_grid <= 0) return;
        const unit = size / heatmap.density_grid;
        const { x, y, width, height } = density.picture;
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(image, x * unit, y * unit, width * unit, height * unit);
        return;
      }
      if (overlay === null) return;
      const computed = getComputedStyle(ctx.canvas);
      const quiet = computed.getPropertyValue('--fg-3').trim() || '#8a8f98';
      const colours = focus.map((_, index) => computed.getPropertyValue(patternPlayerToken(index)).trim() || '#a78bfa');
      const loud = colours[0] ?? (computed.getPropertyValue('--chart-1').trim() || '#a78bfa');
      ctx.lineJoin = 'round';
      ctx.lineCap = 'round';
      // Quiet paths first so the players in focus sit on top.
      for (const pass of ['quiet', 'loud'] as const) {
        for (const round of overlay.rounds) {
          for (const track of round.tracks) {
            const index = focus.indexOf(track.steamid64);
            const strong = focus.length === 0 || index >= 0;
            if ((pass === 'loud') !== strong) continue;
            const own = index >= 0 ? colours[index] : undefined;
            ctx.strokeStyle = strong ? (own ?? loud) : quiet;
            ctx.globalAlpha = strong ? 0.55 : 0.12;
            ctx.lineWidth = strong ? 1.75 : 1;
            ctx.beginPath();
            let open = false;
            for (const point of trackInWindow({ track, step: overlay.step_seconds, tramo })) {
              if (split && levelForAltitude(calibration, point[2]) !== level) {
                open = false;
                continue;
              }
              const at = worldToRendered(calibration, point[0], point[1], size);
              if (open) ctx.lineTo(at.x, at.y);
              else ctx.moveTo(at.x, at.y);
              open = true;
            }
            ctx.stroke();
          }
        }
      }
      ctx.globalAlpha = 1;
    },
    [calibration, densities, focus, heatmap, overlay, picture, split, tramo],
  );

  const onLevel = (zone: PatternArea | null, level: RadarLevel): PatternArea | null =>
    zone === null || (split && zone.level !== undefined && zone.level !== level) ? null : zone;

  return (
    <RadarSurface geometry={geometry} level={levels[0] ?? RADAR_LEVELS.default} levels={levels} paint={paint} label="Radar con las rondas que cumplen el patrón">
      {(projection, zoomed, level) => (
        <DrawLayer
          geometry={geometry}
          projection={projection}
          zoomed={zoomed}
          tool={tool}
          zone={onLevel(area, level)}
          others={others.flatMap((zone) => {
            const shown = onLevel(zone, level);
            return shown === null ? [] : [shown];
          })}
          onBox={(box) => onArea(box === null ? null : areaOfBox({ box, level, twoFloors: multiLevel }))}
          onPolygon={(points) => onArea(areaOfPolygon({ points, level, twoFloors: multiLevel }))}
        />
      )}
    </RadarSurface>
  );
}

type Shown = { x: number; y: number; width: number; height: number };

type DrawProps = {
  geometry: TacticalGeometry;
  projection: RadarProjection;
  zoomed: boolean;
  tool: ZoneTool;
  zone: PatternArea | null;
  others: readonly PatternArea[];
  onBox: (box: WorldBox | null) => void;
  onPolygon: (points: [number, number][]) => void;
};

function DrawLayer({ geometry, projection, zoomed, tool, zone, others, onBox, onPolygon }: DrawProps): ReactNode {
  const start = useRef<RadarPoint | null>(null);
  const [drag, setDrag] = useState<{ a: RadarPoint; b: RadarPoint } | null>(null);
  const [corners, setCorners] = useState<RadarPoint[]>([]);
  const [hover, setHover] = useState<RadarPoint | null>(null);
  const { size, offsetX, offsetY } = projection;
  const { calibration } = geometry;
  const drawing = corners.length > 0;
  useEffect(() => {
    if (!drawing) return;
    const cancel = (event: KeyboardEvent): void => {
      if (event.key === 'Escape') setCorners([]);
    };
    window.addEventListener('keydown', cancel);
    return () => window.removeEventListener('keydown', cancel);
  }, [drawing]);
  useEffect(() => setCorners([]), [tool]);

  const local = (event: PointerEvent<HTMLDivElement>): RadarPoint => {
    const rect = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - rect.left - offsetX, y: event.clientY - rect.top - offsetY };
  };
  const rendered = (world: WorldBox): Shown => {
    const a = worldToRendered(calibration, world.minX, world.maxY, size);
    const b = worldToRendered(calibration, world.maxX, world.minY, size);
    return { x: a.x, y: a.y, width: b.x - a.x, height: b.y - a.y };
  };
  const place = (at: Shown) => ({ left: at.x + offsetX, top: at.y + offsetY, width: at.width, height: at.height });
  const svgPoints = (points: readonly RadarPoint[]): string => points.map((point) => `${point.x + offsetX},${point.y + offsetY}`).join(' ');
  const polygonOf = (area: PatternArea): RadarPoint[] => (area.polygon ?? []).map(([x, y]) => worldToRendered(calibration, x, y, size));
  const finish = (points: readonly RadarPoint[]): void => {
    setCorners([]);
    onPolygon(
      points.map((point): [number, number] => {
        const world = renderedToWorld(calibration, point.x, point.y, size);
        return [world.x, world.y];
      }),
    );
  };
  const addCorner = (at: RadarPoint): void => {
    const first = corners[0];
    const last = corners.at(-1);
    const closes = first !== undefined && corners.length >= 3 && Math.hypot(at.x - first.x, at.y - first.y) < CLOSE_PX;
    // The second click of a double click lands on the last corner and closes too.
    const repeats = last !== undefined && corners.length >= 3 && Math.hypot(at.x - last.x, at.y - last.y) < DRAG_MIN;
    if (closes || repeats) finish(corners);
    else if (corners.length < PATTERN_MAX_POLYGON_POINTS) setCorners([...corners, at]);
  };

  const box = zone === null ? null : boxOfArea(zone);
  let shown: Shown | null = box === null ? null : rendered(box);
  if (drag !== null) {
    shown = { x: Math.min(drag.a.x, drag.b.x), y: Math.min(drag.a.y, drag.b.y), width: Math.abs(drag.a.x - drag.b.x), height: Math.abs(drag.a.y - drag.b.y) };
  }
  const otherBoxes = others.flatMap((entry) => {
    const other = boxOfArea(entry);
    return other === null ? [] : [other];
  });
  const otherPolygons = others.filter((entry) => entry.polygon !== undefined);
  const live = hover === null ? corners : [...corners, hover];

  return (
    <div
      aria-hidden
      className={cn('absolute inset-0 touch-none select-none', zoomed || tool === 'none' ? '' : 'cursor-crosshair')}
      onPointerDown={(event) => {
        if (tool === 'none' || event.button !== 0 || (zoomed && !event.shiftKey)) return;
        event.stopPropagation();
        if (tool === 'polygon') {
          addCorner(local(event));
          return;
        }
        event.currentTarget.setPointerCapture(event.pointerId);
        start.current = local(event);
      }}
      onPointerMove={(event) => {
        if (tool === 'polygon' && drawing) setHover(local(event));
        if (start.current !== null) setDrag({ a: start.current, b: local(event) });
      }}
      onPointerUp={(event) => {
        const from = start.current;
        start.current = null;
        setDrag(null);
        if (from === null) return;
        event.stopPropagation();
        const to = local(event);
        onBox(Math.hypot(to.x - from.x, to.y - from.y) < DRAG_MIN ? null : dragWorldBox(calibration, size, from, to));
      }}
      onPointerCancel={() => {
        start.current = null;
        setDrag(null);
      }}
      onContextMenu={(event) => {
        if (!drawing) return;
        event.preventDefault();
        setCorners([]);
      }}
    >
      {otherBoxes.map((other) => (
        <div key={`${other.minX}:${other.minY}:${other.maxX}:${other.maxY}`} className="pointer-events-none absolute border border-dashed border-fg-3" style={place(rendered(other))} />
      ))}
      {shown === null ? null : <div className="pointer-events-none absolute border border-dashed border-primary bg-primary/8" style={place(shown)} />}
      <svg className="pointer-events-none absolute inset-0 size-full overflow-visible">
        {otherPolygons.map((entry) => (
          <polygon key={JSON.stringify(entry.polygon)} points={svgPoints(polygonOf(entry))} className="fill-none stroke-fg-3" strokeDasharray="4 3" />
        ))}
        {zone?.polygon === undefined || drawing ? null : (
          <polygon points={svgPoints(polygonOf(zone))} className="fill-primary/10 stroke-primary" strokeDasharray="4 3" />
        )}
        {drawing ? <polyline points={svgPoints(live)} className="fill-none stroke-primary" strokeDasharray="4 3" /> : null}
        {corners.map((corner, index) => (
          <circle key={index} cx={corner.x + offsetX} cy={corner.y + offsetY} r={index === 0 ? 4 : 2.5} className="fill-primary" />
        ))}
      </svg>
    </div>
  );
}


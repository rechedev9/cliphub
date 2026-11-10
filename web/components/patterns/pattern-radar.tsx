'use client';

import { useCallback, useMemo, useRef, useState, type PointerEvent, type ReactNode } from 'react';
import type { Heatmap, Overlay } from '@/lib/api/spatial';
import type { PatternArea } from '@/lib/api/patterns';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry, type TacticalSide } from '@/lib/api/tactical';
import { dragWorldBox, type WorldBox } from '@/lib/analysis/grenade-patterns';
import { boxOfArea, patternPlayerToken, trackInWindow } from '@/lib/analysis/pattern-finder';
import { densityPicture, heatBandwidth, heatDensity, SMOOTHING_DEFAULT } from '@/lib/heat-density';
import type { Tramo } from '@/lib/replay/tramo';
import { heatmapSide, type RadarProjection } from '@/lib/tactical-spatial';
import { levelForAltitude, worldToRendered, type RadarPoint } from '@/lib/tactical-transform';
import { cn } from '@/lib/utils';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { densityImage } from '@/components/tactical/spatial/heatmap-view';

/** A drag shorter than this many pixels is a click and clears the zone. */
const DRAG_MIN = 4;

export type PatternPicture = 'recorridos' | 'calor';

type Props = {
  geometry: TacticalGeometry;
  levels: readonly RadarLevel[];
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
  onArea: (box: WorldBox | null, level: RadarLevel) => void;
};

/**
 * The matched rounds on the radar: the paths of the players in focus in the
 * window, everyone else translucent, or their smooth heatmap. Dragging draws
 * the zone of the block being edited (Shift + drag once zoomed).
 */
export function PatternRadar({ geometry, levels, picture, overlay, heatmap, side, focus, tramo, area, others, onArea }: Props): ReactNode {
  const split = levels.length > 1;
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
            for (const point of trackInWindow(track, overlay.step_seconds, tramo)) {
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

  const onLevel = (zone: PatternArea | null, level: RadarLevel): WorldBox | null =>
    zone === null || (split && zone.level !== undefined && zone.level !== level) ? null : boxOfArea(zone);

  return (
    <RadarSurface geometry={geometry} level={levels[0] ?? RADAR_LEVELS.default} levels={levels} paint={paint} label="Radar con las rondas que cumplen el patrón">
      {(projection, zoomed, level) => (
        <DragLayer
          geometry={geometry}
          projection={projection}
          zoomed={zoomed}
          box={onLevel(area, level)}
          others={others.flatMap((zone) => {
            const box = onLevel(zone, level);
            return box === null ? [] : [box];
          })}
          onBox={(box) => onArea(box, level)}
        />
      )}
    </RadarSurface>
  );
}

type Shown = { x: number; y: number; width: number; height: number };

function DragLayer({
  geometry,
  projection,
  zoomed,
  box,
  others,
  onBox,
}: {
  geometry: TacticalGeometry;
  projection: RadarProjection;
  zoomed: boolean;
  box: WorldBox | null;
  others: readonly WorldBox[];
  onBox: (box: WorldBox | null) => void;
}): ReactNode {
  const start = useRef<RadarPoint | null>(null);
  const [drag, setDrag] = useState<{ a: RadarPoint; b: RadarPoint } | null>(null);
  const { size, offsetX, offsetY } = projection;
  const local = (event: PointerEvent<HTMLDivElement>): RadarPoint => {
    const rect = event.currentTarget.getBoundingClientRect();
    return { x: event.clientX - rect.left - offsetX, y: event.clientY - rect.top - offsetY };
  };
  const rendered = (zone: WorldBox): Shown => {
    const a = worldToRendered(geometry.calibration, zone.minX, zone.maxY, size);
    const b = worldToRendered(geometry.calibration, zone.maxX, zone.minY, size);
    return { x: a.x, y: a.y, width: b.x - a.x, height: b.y - a.y };
  };
  let shown: Shown | null = box === null ? null : rendered(box);
  if (drag !== null) {
    shown = { x: Math.min(drag.a.x, drag.b.x), y: Math.min(drag.a.y, drag.b.y), width: Math.abs(drag.a.x - drag.b.x), height: Math.abs(drag.a.y - drag.b.y) };
  }
  const place = (zone: Shown) => ({ left: zone.x + offsetX, top: zone.y + offsetY, width: zone.width, height: zone.height });

  return (
    <div
      aria-hidden
      className={cn('absolute inset-0 touch-none select-none', zoomed ? '' : 'cursor-crosshair')}
      onPointerDown={(event) => {
        if (event.button !== 0 || (zoomed && !event.shiftKey)) return;
        event.stopPropagation();
        event.currentTarget.setPointerCapture(event.pointerId);
        start.current = local(event);
      }}
      onPointerMove={(event) => {
        if (start.current !== null) setDrag({ a: start.current, b: local(event) });
      }}
      onPointerUp={(event) => {
        const from = start.current;
        start.current = null;
        setDrag(null);
        if (from === null) return;
        event.stopPropagation();
        const to = local(event);
        onBox(Math.hypot(to.x - from.x, to.y - from.y) < DRAG_MIN ? null : dragWorldBox(geometry.calibration, size, from, to));
      }}
      onPointerCancel={() => {
        start.current = null;
        setDrag(null);
      }}
    >
      {others.map((zone) => (
        <div key={`${zone.minX}:${zone.minY}:${zone.maxX}:${zone.maxY}`} className="pointer-events-none absolute border border-dashed border-fg-3" style={place(rendered(zone))} />
      ))}
      {shown === null ? null : <div className="pointer-events-none absolute border border-dashed border-primary bg-primary/8" style={place(shown)} />}
    </div>
  );
}

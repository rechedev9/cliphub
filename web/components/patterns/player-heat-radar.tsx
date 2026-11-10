'use client';

import { useCallback, useMemo, useRef, type ReactNode } from 'react';
import type { PatternArea } from '@/lib/api/patterns';
import { RADAR_LEVELS, type RadarLevel, type TacticalGeometry } from '@/lib/api/tactical';
import type { WorldBox } from '@/lib/analysis/grenade-patterns';
import { areaOfBox } from '@/lib/analysis/pattern-finder';
import { densityPicture } from '@/lib/heat-density';
import type { DensityPicture, HeatDensity } from '@/lib/heat-density';
import { densityImage } from '@/components/tactical/spatial/heatmap-view';
import { RadarSurface, type RadarFrame } from '@/components/tactical/spatial/radar-surface';
import { DrawLayer } from './pattern-radar';

/** One player's heat on every floor, in the colour token that player carries everywhere on the canvas. */
export type PlayerHeat = { token: string; byLevel: ReadonlyMap<RadarLevel, HeatDensity> };

/** Below this share of a player's peak a square stays clear, so the floor shows through. */
const FLOOR_SHARE = 0.05;
const TOP_ALPHA = 0.88;

/** The density as an alpha mask on a grid-sized canvas; the colour is applied when it is drawn. */
function maskOf(cache: WeakMap<HeatDensity, HTMLCanvasElement>, density: HeatDensity): HTMLCanvasElement | null {
  const known = cache.get(density);
  if (known !== undefined) return known;
  const { grid, mass, peak } = density;
  if (!(peak > 0)) return null;
  const canvas = document.createElement('canvas');
  canvas.width = grid;
  canvas.height = grid;
  const ctx = canvas.getContext('2d');
  if (ctx === null) return null;
  const image = ctx.createImageData(grid, grid);
  for (let index = 0; index < grid * grid; index += 1) {
    const share = (mass[index] ?? 0) / peak;
    image.data[index * 4 + 3] = share < FLOOR_SHARE ? 0 : Math.round(Math.min(1, share ** 0.7) * TOP_ALPHA * 255);
  }
  ctx.putImageData(image, 0, 0);
  cache.set(density, canvas);
  return canvas;
}

/**
 * The radar of the Jugadores panel. One player, or the team, reads on the heat scale of every other
 * heatmap: blue where they barely are, purple, pink where they stay most. Several players each take
 * their own colour, more opaque where they spent more time. Dragging draws the box that keeps the heat
 * and the rounds inside it.
 */
export function PlayerHeatRadar({
  geometry,
  levels,
  multiLevel,
  heats,
  box,
  boxLevel,
  onBox,
}: {
  geometry: TacticalGeometry;
  levels: readonly RadarLevel[];
  multiLevel: boolean;
  heats: readonly PlayerHeat[];
  box: WorldBox | null;
  boxLevel: RadarLevel;
  onBox: (box: { box: WorldBox; level: RadarLevel } | null) => void;
}): ReactNode {
  const masks = useRef(new WeakMap<HeatDensity, HTMLCanvasElement>());
  const images = useRef(new WeakMap<Uint8ClampedArray<ArrayBuffer>, HTMLCanvasElement>());
  const scratch = useRef<HTMLCanvasElement | null>(null);
  // A single series reads on the heat scale, both floors on one peak; several keep their own colours.
  const scale = useMemo((): ReadonlyMap<RadarLevel, DensityPicture> | null => {
    const only = heats.length === 1 ? heats[0] : undefined;
    if (only === undefined) return null;
    const peak = Math.max(0, ...[...only.byLevel.values()].map((density) => density.peak));
    const out = new Map<RadarLevel, DensityPicture>();
    for (const [level, density] of only.byLevel) {
      const picture = densityPicture(density, peak);
      if (picture !== null) out.set(level, picture);
    }
    return out;
  }, [heats]);

  const paint = useCallback(
    ({ ctx, size, level }: RadarFrame) => {
      if (scale !== null) {
        const picture = scale.get(level);
        const density = heats[0]?.byLevel.get(level);
        const image = picture === undefined ? null : densityImage(images.current, picture);
        if (picture === undefined || image === null || density === undefined) return;
        // The density grid covers the whole radar square, so a square is size / grid pixels.
        const unit = size / density.grid;
        ctx.imageSmoothingEnabled = true;
        ctx.imageSmoothingQuality = 'high';
        ctx.drawImage(image, picture.x * unit, picture.y * unit, picture.width * unit, picture.height * unit);
        return;
      }
      const side = Math.max(1, Math.ceil(size));
      const canvas = scratch.current ?? document.createElement('canvas');
      scratch.current = canvas;
      if (canvas.width !== side) {
        canvas.width = side;
        canvas.height = side;
      }
      const layer = canvas.getContext('2d');
      if (layer === null) return;
      const computed = getComputedStyle(ctx.canvas);
      for (const heat of heats) {
        const density = heat.byLevel.get(level);
        const mask = density === undefined ? null : maskOf(masks.current, density);
        if (mask === null) continue;
        layer.globalCompositeOperation = 'source-over';
        layer.clearRect(0, 0, side, side);
        layer.imageSmoothingEnabled = true;
        layer.imageSmoothingQuality = 'high';
        layer.drawImage(mask, 0, 0, side, side);
        layer.globalCompositeOperation = 'source-in';
        layer.fillStyle = computed.getPropertyValue(heat.token).trim() || '#a78bfa';
        layer.fillRect(0, 0, side, side);
        ctx.drawImage(canvas, 0, 0, size, size);
      }
    },
    [heats, scale],
  );

  const zone = (shown: RadarLevel): PatternArea | null =>
    box === null || (multiLevel && boxLevel !== shown) ? null : areaOfBox({ box, level: boxLevel, twoFloors: multiLevel });

  return (
    <RadarSurface geometry={geometry} level={levels[0] ?? RADAR_LEVELS.default} levels={levels} paint={paint} label="Radar con el calor de cada jugador">
      {(projection, zoomed, level) => (
        <DrawLayer
          geometry={geometry}
          projection={projection}
          zoomed={zoomed}
          tool="box"
          zone={zone(level)}
          others={[]}
          onBox={(next) => onBox(next === null ? null : { box: next, level })}
          onPolygon={() => undefined}
        />
      )}
    </RadarSurface>
  );
}

import type { RadarRect } from '../tactical-transform.ts';

/**
 * Zoom and pan of the 2D map as a window onto the radar square (fractions of
 * it, like `radarViewRect`). The fit itself stays `radarProjection`'s, so the
 * map, the players and the drawings move as one.
 */

/** The closest the wheel zooms in, as a multiple of the whole map. */
export const MAX_ZOOM = 8;

/** Mirrors `radarProjection`: how a window sits in a box. */
function fit(box: { width: number; height: number }, view: RadarRect): { size: number; offsetX: number; offsetY: number } {
  const size = Math.max(1, Math.min(box.width / view.width, box.height / view.height));
  return {
    size,
    offsetX: (box.width - size * view.width) / 2 - size * view.x,
    offsetY: (box.height - size * view.height) / 2 - size * view.y,
  };
}

export function zoomLevel(view: RadarRect, base: RadarRect): number {
  return base.width / view.width;
}

/** Keeps the window's centre over the base window, so the map can never be panned away. */
export function clampView(view: RadarRect, base: RadarRect): RadarRect {
  if (view.width >= base.width && view.height >= base.height) return base;
  const cx = Math.min(base.x + base.width, Math.max(base.x, view.x + view.width / 2));
  const cy = Math.min(base.y + base.height, Math.max(base.y, view.y + view.height / 2));
  return { x: cx - view.width / 2, y: cy - view.height / 2, width: view.width, height: view.height };
}

/**
 * The window after zooming by `factor` (above 1 zooms in) around `cursor`, a
 * point of the box in CSS pixels that stays over the same spot of the map.
 */
export function zoomViewAt(options: {
  box: { width: number; height: number };
  view: RadarRect;
  base: RadarRect;
  cursor: { x: number; y: number };
  factor: number;
}): RadarRect {
  const { box, view, base, cursor, factor } = options;
  const current = fit(box, view);
  const mapX = (cursor.x - current.offsetX) / current.size;
  const mapY = (cursor.y - current.offsetY) / current.size;
  const level = Math.min(MAX_ZOOM, Math.max(1, zoomLevel(view, base) * factor));
  if (level <= 1) return base;
  const width = base.width / level;
  const height = base.height / level;
  const size = Math.max(1, Math.min(box.width / width, box.height / height));
  const x = ((box.width - size * width) / 2 - cursor.x + size * mapX) / size;
  const y = ((box.height - size * height) / 2 - cursor.y + size * mapY) / size;
  return clampView({ x, y, width, height }, base);
}

/** The window after dragging the map by `dx`, `dy` CSS pixels. */
export function panView(options: { box: { width: number; height: number }; view: RadarRect; base: RadarRect; dx: number; dy: number }): RadarRect {
  const { box, view, base, dx, dy } = options;
  if (zoomLevel(view, base) <= 1) return base;
  const { size } = fit(box, view);
  return clampView({ ...view, x: view.x - dx / size, y: view.y - dy / size }, base);
}

/**
 * The window that shows a world rectangle already in square fractions, grown by
 * `margin` of its longer side so the outermost corridors keep some air.
 */
export function paddedView(rect: RadarRect, margin: number): RadarRect {
  const pad = Math.max(rect.width, rect.height) * margin;
  return { x: rect.x - pad, y: rect.y - pad, width: rect.width + 2 * pad, height: rect.height + 2 * pad };
}

/** Zoom a followed player starts at, as a multiple of the whole map. */
export const FOLLOW_ZOOM = 2.5;

/**
 * The view that keeps a followed player (a point on the radar square) in sight:
 * zoomed in to at least `FOLLOW_ZOOM`, recentred once the player leaves the
 * middle `keep` share of the view, so the map moves in steps instead of every frame.
 */
export function followView(options: { view: RadarRect; base: RadarRect; point: { x: number; y: number }; keep?: number }): RadarRect {
  const { view, base, point } = options;
  const keep = options.keep ?? 0.5;
  let next = view;
  if (zoomLevel(view, base) < FOLLOW_ZOOM) next = { x: 0, y: 0, width: base.width / FOLLOW_ZOOM, height: base.height / FOLLOW_ZOOM };
  else {
    const inner = { x: view.x + (view.width * (1 - keep)) / 2, y: view.y + (view.height * (1 - keep)) / 2, width: view.width * keep, height: view.height * keep };
    const inside = point.x >= inner.x && point.x <= inner.x + inner.width && point.y >= inner.y && point.y <= inner.y + inner.height;
    if (inside) return view;
  }
  return clampView({ x: point.x - next.width / 2, y: point.y - next.height / 2, width: next.width, height: next.height }, base);
}

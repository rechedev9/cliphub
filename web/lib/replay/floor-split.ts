import { RADAR_LEVELS } from '../api/tactical.ts';
import type { RadarLevel, TacticalGeometry } from '../api/tactical.ts';
import type { RadarProjection } from '../tactical-spatial.ts';
import { isMultiLevelMap, levelForAltitude, radarViewRect } from '../tactical-transform.ts';
import type { RadarRect, WorldMark } from '../tactical-transform.ts';
import { followView, panView, zoomViewAt } from './map-view.ts';

/**
 * The shared floor layout of a two-floor map (Nuke, Vertigo), the one the
 * replay canvas and the heatmap, finder and pattern surfaces arrange floors by.
 * Each floor is cropped to its own extent and both are drawn at one scale: the
 * upper floor large at the left, the lower one as a second block at the right
 * with a gap between them (stacked only in a portrait box). Zoom and pan are
 * one `FloorView` that moves both floors by the same world distance.
 */

/** One floor's block of the canvas, in CSS pixels, with the floor's extent on the radar square. */
export type FloorPane = { level: RadarLevel; x: number; y: number; width: number; height: number; rect: RadarRect };

/** The gap between the two blocks, in CSS pixels. */
export const PANE_GAP = 28;

/** Upper floor first, as the game's own radar lists them. */
export const BOTH_FLOORS: readonly RadarLevel[] = [RADAR_LEVELS.default, RADAR_LEVELS.lower];

/** Each floor's extent as a window of the radar square (fractions, like `radarViewRect`). */
export type FloorRects = Readonly<Record<RadarLevel, RadarRect>>;

export type FloorLayout = {
  panes: FloorPane[];
  /** Radar-square pixels per unit of the square at zoom 1; every pane shares it. */
  scale: number;
  stacked: boolean;
};

/**
 * The panes for `levels` in `box`. One floor takes the whole box. Two are each
 * sized to their own extent at one common scale and centred as a group: side by
 * side when the box is landscape, stacked, upper first, when it is portrait.
 */
export function layoutFloors(options: { box: { width: number; height: number }; rects: FloorRects; levels: readonly RadarLevel[] }): FloorLayout {
  const { rects, levels } = options;
  const width = Math.max(1, options.box.width);
  const height = Math.max(1, options.box.height);
  const first = levels[0] ?? RADAR_LEVELS.default;
  const second = levels[1];
  if (second === undefined) {
    const rect = rects[first];
    return { panes: [{ level: first, x: 0, y: 0, width, height, rect }], scale: Math.min(width / rect.width, height / rect.height), stacked: false };
  }
  const a = rects[first];
  const b = rects[second];
  const stacked = height > width;
  const gap = Math.min(PANE_GAP, (stacked ? height : width) * 0.1);
  const scale = stacked ? Math.min(width / Math.max(a.width, b.width), (height - gap) / (a.height + b.height)) : Math.min((width - gap) / (a.width + b.width), height / Math.max(a.height, b.height));
  const sized = (level: RadarLevel, rect: RadarRect): FloorPane => ({ level, x: 0, y: 0, width: Math.max(1, Math.round(rect.width * scale)), height: Math.max(1, Math.round(rect.height * scale)), rect });
  const one = sized(first, a);
  const two = sized(second, b);
  if (stacked) {
    const top = Math.round((height - one.height - two.height - gap) / 2);
    one.x = Math.round((width - one.width) / 2);
    one.y = top;
    two.x = Math.round((width - two.width) / 2);
    two.y = top + one.height + Math.round(gap);
  } else {
    const left = Math.round((width - one.width - two.width - gap) / 2);
    one.x = left;
    one.y = Math.round((height - one.height) / 2);
    two.x = left + one.width + Math.round(gap);
    two.y = Math.round((height - two.height) / 2);
  }
  return { panes: [one, two], scale, stacked };
}

/** Zoom and pan shared by every pane; `pan` is in radar-square fractions, so both floors move by the same world distance. */
export type FloorView = { zoom: number; panX: number; panY: number };

export const WHOLE_FLOORS: FloorView = { zoom: 1, panX: 0, panY: 0 };

export function isWholeView(view: FloorView): boolean {
  return view.zoom <= 1;
}

/** The part of a floor's extent the view shows; its centre never leaves the extent. */
export function floorWindow(rect: RadarRect, view: FloorView): RadarRect {
  const zoom = Math.max(1, view.zoom);
  if (zoom === 1) return rect;
  const width = rect.width / zoom;
  const height = rect.height / zoom;
  const cx = Math.min(rect.x + rect.width, Math.max(rect.x, rect.x + rect.width / 2 + view.panX));
  const cy = Math.min(rect.y + rect.height, Math.max(rect.y, rect.y + rect.height / 2 + view.panY));
  return { x: cx - width / 2, y: cy - height / 2, width, height };
}

/** The view that makes `window` the part of `rect` on show. */
export function viewOfWindow(rect: RadarRect, window: RadarRect): FloorView {
  const zoom = rect.width / window.width;
  if (!(zoom > 1)) return WHOLE_FLOORS;
  return { zoom, panX: window.x + window.width / 2 - (rect.x + rect.width / 2), panY: window.y + window.height / 2 - (rect.y + rect.height / 2) };
}

/** Where the view puts a pane's floor, in canvas coordinates; the size is the same for every pane. */
export function paneProjection(options: { layout: FloorLayout; pane: FloorPane; view: FloorView }): RadarProjection {
  const { layout, pane, view } = options;
  const size = Math.max(1, Math.floor(layout.scale * Math.max(1, view.zoom)));
  const window = floorWindow(pane.rect, view);
  return {
    size,
    offsetX: Math.round(pane.x + pane.width / 2 - size * (window.x + window.width / 2)),
    offsetY: Math.round(pane.y + pane.height / 2 - size * (window.y + window.height / 2)),
  };
}

/** The same projection relative to the pane's own corner, for a layer laid over just that pane. */
export function localProjection(pane: FloorPane, projection: RadarProjection): RadarProjection {
  return { size: projection.size, offsetX: projection.offsetX - pane.x, offsetY: projection.offsetY - pane.y };
}

/** The pane under a canvas point, with the point relative to it; null in the gap between them. */
export function paneAt(panes: readonly FloorPane[], x: number, y: number): { pane: FloorPane; x: number; y: number } | null {
  const pane = panes.find((entry) => x >= entry.x && x < entry.x + entry.width && y >= entry.y && y < entry.y + entry.height);
  return pane === undefined ? null : { pane, x: x - pane.x, y: y - pane.y };
}

/** The pane under a canvas point, or the closest one when the point is in the gap or outside. */
export function nearestPane(panes: readonly FloorPane[], x: number, y: number): { pane: FloorPane; x: number; y: number } {
  const hit = paneAt(panes, x, y);
  if (hit !== null) return hit;
  let best = panes[0];
  let bestDistance = Number.POSITIVE_INFINITY;
  for (const pane of panes) {
    const dx = Math.max(pane.x - x, 0, x - (pane.x + pane.width));
    const dy = Math.max(pane.y - y, 0, y - (pane.y + pane.height));
    const distance = Math.hypot(dx, dy);
    if (distance < bestDistance) {
      best = pane;
      bestDistance = distance;
    }
  }
  return { pane: best, x: x - best.x, y: y - best.y };
}

/** The view after zooming by `factor` (above 1 zooms in) around a point of `pane`, in the pane's own pixels. */
export function zoomFloors(options: { pane: FloorPane; view: FloorView; cursor: { x: number; y: number }; factor: number }): FloorView {
  const { pane, view, cursor, factor } = options;
  return viewOfWindow(pane.rect, zoomViewAt({ box: pane, view: floorWindow(pane.rect, view), base: pane.rect, cursor, factor }));
}

/** The view after dragging the map by `dx`, `dy` CSS pixels on `pane`. */
export function panFloors(options: { pane: FloorPane; view: FloorView; dx: number; dy: number }): FloorView {
  const { pane, view, dx, dy } = options;
  return viewOfWindow(pane.rect, panView({ box: pane, view: floorWindow(pane.rect, view), base: pane.rect, dx, dy }));
}

/** The view that keeps a followed player, a point of the radar square, in sight on the pane they stand on. */
export function followFloors(options: { pane: FloorPane; view: FloorView; point: { x: number; y: number } }): FloorView {
  const { pane, view, point } = options;
  return viewOfWindow(pane.rect, followView({ view: floorWindow(pane.rect, view), base: pane.rect, point }));
}

export function sameFloorView(a: FloorView, b: FloorView): boolean {
  return a.zoom === b.zoom && a.panX === b.panX && a.panY === b.panY;
}

/** Zoomed views by map for the session, so leaving the 2D view and coming back keeps the framing. */
const viewMemory = new Map<string, FloorView>();

export function rememberedFloorView(map: string): FloorView {
  return viewMemory.get(map) ?? WHOLE_FLOORS;
}

/** Remembers `view` for `map`; the whole map forgets it. */
export function rememberFloorView(map: string, view: FloorView): void {
  if (isWholeView(view)) viewMemory.delete(map);
  else viewMemory.set(map, view);
}

/**
 * Each floor's crop from the plays recorded on it: the played area plus its
 * margin, with `marks` kept inside the floor they stand on. A map with one
 * floor answers the same crop for both.
 */
export function floorRectsOf(geometry: TacticalGeometry, marks: readonly WorldMark[]): FloorRects {
  if (!isMultiLevelMap(geometry.calibration)) {
    const whole = radarViewRect(geometry, marks);
    return { default: whole, lower: whole };
  }
  const of = (level: RadarLevel): RadarRect =>
    radarViewRect(
      { ...geometry, levels: geometry.levels.filter((entry) => entry.name === level) },
      marks.filter((mark) => {
        const z = mark.at(2);
        return z === undefined || levelForAltitude(geometry.calibration, z) === level;
      }),
    );
  return { default: of(RADAR_LEVELS.default), lower: of(RADAR_LEVELS.lower) };
}

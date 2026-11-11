import { NAV_GROW, NAV_RADAR_PALETTE, heightTone, toneColor, toneRange } from '@/lib/replay/nav-radar';
import type { NavPolygon, PlaceRegion } from '@/lib/replay/nav-radar';

/** World units the floor's outline is smoothed by: staircase edges between nav areas turn into straight walls. */
const SMOOTH_UNITS = 11;
/** An enclosed gap in the floor up to this many square world units is an obstacle (crate, silo, pillar) drawn as solid dark grey. */
const OBSTACLE_MAX_AREA = 420000;
/** World units a bombsite or spawn block reaches past its areas, so it reads as one rounded block. */
const BLOCK_PAD = 30;
const BLOCK_RADIUS = 60;

/** The world-to-device mapping of the plate: device pixels per world unit, and the device pixel of world (0, 0). */
export type PlateMatrix = { scale: number; x: number; y: number };

function tracePolygon(ctx: CanvasRenderingContext2D, polygon: NavPolygon): void {
  const { points } = polygon;
  ctx.moveTo(points[0], points[1]);
  for (let i = 2; i < points.length; i += 2) ctx.lineTo(points[i], points[i + 1]);
  ctx.closePath();
}

/** Every area grown by `NAV_GROW` and filled in its height tone, lowest first, with no outlines between areas. */
function paintAreas(ctx: CanvasRenderingContext2D, polygons: readonly NavPolygon[]): void {
  const range = toneRange(polygons);
  ctx.lineJoin = 'round';
  ctx.lineWidth = 2 * NAV_GROW;
  for (const polygon of polygons) {
    const color = toneColor(heightTone(polygon.z, range.minZ, range.maxZ), NAV_RADAR_PALETTE.low, NAV_RADAR_PALETTE.high);
    ctx.beginPath();
    tracePolygon(ctx, polygon);
    ctx.fillStyle = color;
    ctx.strokeStyle = color;
    ctx.fill();
    ctx.stroke();
  }
}

function smoothstep(edge0: number, edge1: number, value: number): number {
  const t = Math.min(1, Math.max(0, (value - edge0) / (edge1 - edge0)));
  return t * t * (3 - 2 * t);
}

/** Marks the empty pixels reachable from the image border; the rest are gaps inside the floor. */
function outsideOf(options: { floor: Uint8Array; width: number; height: number }): Uint8Array {
  const { floor, width, height } = options;
  const outside = new Uint8Array(width * height);
  const stack: number[] = [];
  const push = (at: number): void => {
    if (floor[at] === 0 && outside[at] === 0) {
      outside[at] = 1;
      stack.push(at);
    }
  };
  for (let x = 0; x < width; x += 1) {
    push(x);
    push((height - 1) * width + x);
  }
  for (let y = 0; y < height; y += 1) {
    push(y * width);
    push(y * width + width - 1);
  }
  while (stack.length > 0) {
    const at = stack.pop() ?? 0;
    const x = at % width;
    if (x > 0) push(at - 1);
    if (x < width - 1) push(at + 1);
    if (at >= width) push(at - width);
    if (at < width * (height - 1)) push(at + width);
  }
  return outside;
}

/** Marks the enclosed gaps of at most `maxPixels` pixels. */
function smallGaps(options: { floor: Uint8Array; outside: Uint8Array; width: number; height: number; maxPixels: number }): Uint8Array {
  const { floor, outside, width, height, maxPixels } = options;
  const gaps = new Uint8Array(width * height);
  const seen = new Uint8Array(width * height);
  for (let start = 0; start < floor.length; start += 1) {
    if (floor[start] !== 0 || outside[start] !== 0 || seen[start] !== 0) continue;
    const members: number[] = [start];
    seen[start] = 1;
    for (let head = 0; head < members.length; head += 1) {
      const at = members[head];
      const x = at % width;
      const next = [x > 0 ? at - 1 : -1, x < width - 1 ? at + 1 : -1, at >= width ? at - width : -1, at < width * (height - 1) ? at + width : -1];
      for (const n of next) {
        if (n >= 0 && floor[n] === 0 && seen[n] === 0) {
          seen[n] = 1;
          members.push(n);
        }
      }
    }
    if (members.length <= maxPixels) for (const at of members) gaps[at] = 1;
  }
  return gaps;
}

/**
 * Turns the blurred floor into the plate: a smooth antialiased outline, and the
 * small enclosed gaps filled as solid obstacle blocks instead of black cut-outs.
 */
function shapeFloor(options: { image: ImageData; maxGapPixels: number }): void {
  const { image, maxGapPixels } = options;
  const { data, width, height } = image;
  const floor = new Uint8Array(width * height);
  for (let i = 0; i < floor.length; i += 1) floor[i] = data[i * 4 + 3] >= 128 ? 1 : 0;
  const gaps = smallGaps({ floor, outside: outsideOf({ floor, width, height }), width, height, maxPixels: maxGapPixels });
  const obstacle = NAV_RADAR_PALETTE.obstacle;
  for (let i = 0; i < floor.length; i += 1) {
    const o = i * 4;
    const alpha = data[o + 3] / 255;
    const x = i % width;
    const nearGap = gaps[i] === 1 || (x > 0 && gaps[i - 1] === 1) || (x < width - 1 && gaps[i + 1] === 1) || (i >= width && gaps[i - width] === 1) || (i < width * (height - 1) && gaps[i + width] === 1);
    if (!nearGap) {
      data[o + 3] = Math.round(smoothstep(0.3, 0.55, alpha) * 255);
      continue;
    }
    const t = smoothstep(0.25, 0.75, alpha);
    const solid = alpha > 0.1;
    for (let c = 0; c < 3; c += 1) data[o + c] = Math.round(obstacle[c] + ((solid ? data[o + c] : obstacle[c]) - obstacle[c]) * t);
    data[o + 3] = 255;
  }
}

function newCanvas(width: number, height: number): { canvas: HTMLCanvasElement; ctx: CanvasRenderingContext2D } | null {
  const canvas = document.createElement('canvas');
  canvas.width = width;
  canvas.height = height;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  return ctx === null ? null : { canvas, ctx };
}

/**
 * Paints one floor's plate onto `target` (width x height device pixels): the
 * union of the grown nav areas in height tones, blurred and thresholded so the
 * walls come out smooth and straight, small enclosed gaps filled as dark
 * obstacle blocks, and each bombsite or spawn as one tinted rounded block.
 */
export function paintPlate(options: { target: CanvasRenderingContext2D; width: number; height: number; polygons: readonly NavPolygon[]; regions: readonly PlaceRegion[]; matrix: PlateMatrix }): void {
  const { target, width, height, polygons, regions, matrix } = options;
  if (polygons.length === 0) return;
  const sigma = Math.min(40, Math.max(0.7, SMOOTH_UNITS * matrix.scale));
  const pad = Math.ceil(sigma * 3) + 2;
  const sketch = newCanvas(width + 2 * pad, height + 2 * pad);
  const smooth = newCanvas(width + 2 * pad, height + 2 * pad);
  if (sketch === null || smooth === null) return;
  sketch.ctx.setTransform(matrix.scale, 0, 0, -matrix.scale, matrix.x + pad, matrix.y + pad);
  paintAreas(sketch.ctx, polygons);
  smooth.ctx.filter = `blur(${sigma}px)`;
  smooth.ctx.drawImage(sketch.canvas, 0, 0);
  smooth.ctx.filter = 'none';
  const image = smooth.ctx.getImageData(0, 0, smooth.canvas.width, smooth.canvas.height);
  shapeFloor({ image, maxGapPixels: OBSTACLE_MAX_AREA * matrix.scale * matrix.scale });
  smooth.ctx.putImageData(image, 0, 0);
  smooth.ctx.setTransform(matrix.scale, 0, 0, -matrix.scale, matrix.x + pad, matrix.y + pad);
  smooth.ctx.globalCompositeOperation = 'source-atop';
  for (const region of regions) {
    smooth.ctx.fillStyle = region.color;
    smooth.ctx.beginPath();
    smooth.ctx.roundRect(region.minX - BLOCK_PAD, region.minY - BLOCK_PAD, region.maxX - region.minX + 2 * BLOCK_PAD, region.maxY - region.minY + 2 * BLOCK_PAD, BLOCK_RADIUS);
    smooth.ctx.fill();
  }
  target.setTransform(1, 0, 0, 1, 0, 0);
  target.drawImage(smooth.canvas, -pad, -pad);
}

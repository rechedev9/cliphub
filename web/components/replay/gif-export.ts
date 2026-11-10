import { GIFEncoder, applyPalette, quantize } from 'gifenc';
import type { GifPalette } from 'gifenc';
import type { Stroke } from '@/lib/api/spatial';
import type { RadarCalibration, RadarLevel } from '@/lib/api/tactical';
import { GIF_COLORS, GIF_TRANSPARENT, gifPalette, gifSize, keepChanged, paletteSample } from '@/lib/replay/replay-gif';
import type { GifFrame } from '@/lib/replay/replay-gif';
import { arrowHeadPoints } from '@/lib/tactical-spatial';
import type { RadarProjection } from '@/lib/tactical-spatial';
import { worldToRendered } from '@/lib/tactical-transform';
import { STROKE_VAR, onLevel, strokeWidthPx } from '@/components/tactical/spatial/telestrator';

/** What the map hands the GIF: a way to draw any moment off screen, framed as it is on screen. */
export type MapCapture = {
  box: { width: number; height: number };
  calibration: RadarCalibration;
  /** A design token's value on the map, such as `--side-ct`. */
  token: (name: string) => string;
  fontFamily: string;
  /**
   * The map at `seconds` on the transport's clock, at the visible canvas's pixel size, with each
   * floor pane's projection for the drawings; null when it cannot draw. Called in frame order.
   */
  paint: (seconds: number) => { canvas: HTMLCanvasElement; panes: MapPane[] } | null;
};

/** One floor's pane of a frame; `level` is undefined when one pane shows the whole map. */
export type MapPane = { level: RadarLevel | undefined; projection: RadarProjection };

/** What goes over the map in each frame: the drawings on screen at that moment and the HUD's line. */
export type GifOverlay = { strokesAt: (seconds: number) => readonly Stroke[]; captionAt: (seconds: number) => string };

const PALETTE_FRAMES = 5;
const PALETTE_PIXELS = 400_000;
/** The encoder gives the page back this often, so the progress and Cancelar keep answering. */
const YIELD_MS = 30;
const HUD_BACKGROUND = 'rgba(11, 12, 14, 0.85)';
const HUD_TEXT = '#ffffff';

function cssColor(value: string, capture: MapCapture): string {
  const token = /^var\((--[\w-]+)\)$/.exec(value)?.[1];
  if (token === undefined) return value;
  const resolved = capture.token(token);
  return resolved === '' ? HUD_TEXT : resolved;
}

function drawStrokes(ctx: CanvasRenderingContext2D, strokes: readonly Stroke[], frame: { capture: MapCapture; pane: MapPane }): void {
  const { capture, pane } = frame;
  const { calibration } = capture;
  const { projection } = pane;
  for (const stroke of strokes) {
    if (!onLevel(stroke, pane.level)) continue;
    const points = stroke.points.map(([x, y]) => {
      const p = worldToRendered(calibration, x, y, projection.size);
      return { x: p.x + projection.offsetX, y: p.y + projection.offsetY };
    });
    const last = points.at(-1);
    const before = points.at(-2);
    if (last === undefined || before === undefined) continue;
    const color = cssColor(STROKE_VAR[stroke.color], capture);
    const width = strokeWidthPx(stroke, projection);
    ctx.strokeStyle = color;
    ctx.fillStyle = color;
    ctx.lineWidth = width;
    ctx.lineCap = 'round';
    ctx.lineJoin = 'round';
    ctx.beginPath();
    for (const [index, point] of points.entries()) {
      if (index === 0) ctx.moveTo(point.x, point.y);
      else ctx.lineTo(point.x, point.y);
    }
    ctx.stroke();
    if (!stroke.arrow) continue;
    ctx.beginPath();
    for (const corner of arrowHeadPoints(before, last, width * 4)) ctx.lineTo(corner.x, corner.y);
    ctx.closePath();
    ctx.fill();
  }
}

function drawCaption(ctx: CanvasRenderingContext2D, text: string, capture: MapCapture): void {
  ctx.font = `600 13px ${capture.fontFamily}`;
  ctx.textBaseline = 'middle';
  const width = ctx.measureText(text).width;
  ctx.fillStyle = HUD_BACKGROUND;
  ctx.beginPath();
  ctx.roundRect(8, 8, width + 16, 24, 4);
  ctx.fill();
  ctx.fillStyle = HUD_TEXT;
  ctx.fillText(text, 16, 20);
}

/** One canvas of `size` that `paint` fills with the map at a moment, the drawings and the HUD's line; `size.scale` maps the box into it. */
export function framePainter(options: {
  capture: MapCapture;
  overlay: GifOverlay;
  size: { width: number; height: number; scale: number };
}): { canvas: HTMLCanvasElement; ctx: CanvasRenderingContext2D; paint: (seconds: number) => void } {
  const { capture, overlay, size } = options;
  const canvas = document.createElement('canvas');
  canvas.width = size.width;
  canvas.height = size.height;
  const ctx = canvas.getContext('2d', { willReadFrequently: true });
  if (ctx === null) throw new Error('2D canvas unavailable');
  ctx.imageSmoothingQuality = 'high';
  const paint = (seconds: number): void => {
    const map = capture.paint(seconds);
    if (map === null) throw new Error('the map could not draw');
    ctx.setTransform(1, 0, 0, 1, 0, 0);
    ctx.drawImage(map.canvas, 0, 0, size.width, size.height);
    ctx.setTransform(size.scale, 0, 0, size.scale, 0, 0);
    const strokes = overlay.strokesAt(seconds);
    for (const pane of map.panes) drawStrokes(ctx, strokes, { capture, pane });
    drawCaption(ctx, overlay.captionAt(seconds), capture);
  };
  return { canvas, ctx, paint };
}

export function nextTask(): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

/** The palette from a few frames across the span, so the utility and the drawings of later frames have their colours. */
function pickPalette(frames: readonly GifFrame[], frameAt: (seconds: number) => Uint8ClampedArray): GifPalette {
  const picks = new Set<number>();
  for (let i = 0; i < PALETTE_FRAMES; i++) picks.add(Math.round((i * (frames.length - 1)) / (PALETTE_FRAMES - 1)));
  const samples: Uint8ClampedArray[] = [];
  for (const index of picks) {
    const frame = frames[index];
    if (frame !== undefined) samples.push(frameAt(frame.seconds));
  }
  const pixels = samples.reduce((sum, sample) => sum + sample.length / 4, 0);
  return quantize(paletteSample(samples, Math.ceil(pixels / PALETTE_PIXELS)), GIF_COLORS, { format: 'rgb565' });
}

/**
 * Draws every frame off screen, with the drawings and the HUD's line over the
 * map, and encodes the GIF. Each frame after the first only writes the pixels
 * that changed, so the still map costs almost nothing. Throws when `signal` aborts.
 */
export async function encodeReplayGif(options: {
  capture: MapCapture;
  frames: readonly GifFrame[];
  overlay: GifOverlay;
  onProgress: (done: number) => void;
  signal: AbortSignal;
}): Promise<Uint8Array<ArrayBuffer>> {
  const { capture, frames, overlay, onProgress, signal } = options;
  const size = gifSize(capture.box);
  const painter = framePainter({ capture, overlay, size });
  const frameAt = (seconds: number): Uint8ClampedArray => {
    painter.paint(seconds);
    return painter.ctx.getImageData(0, 0, size.width, size.height).data;
  };

  const colors = pickPalette(frames, frameAt);
  const table = gifPalette(colors);
  const gif = GIFEncoder();
  let previous: Uint8Array | null = null;
  let lastYield = performance.now();
  for (const [done, frame] of frames.entries()) {
    if (signal.aborted) throw new Error('cancelled');
    const index = applyPalette(frameAt(frame.seconds), colors, 'rgb565');
    if (previous === null) gif.writeFrame(index, size.width, size.height, { palette: table, delay: frame.delayMs, repeat: 0 });
    else gif.writeFrame(keepChanged(index, previous), size.width, size.height, { delay: frame.delayMs, transparent: true, transparentIndex: GIF_TRANSPARENT, dispose: 1 });
    previous = index;
    onProgress(done + 1);
    if (performance.now() - lastYield > YIELD_MS) {
      await nextTask();
      lastYield = performance.now();
    }
  }
  gif.finish();
  return gif.bytes();
}

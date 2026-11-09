import { TACTICAL_EVENT_KINDS } from '../api/tactical.ts';
import type { TimelineEvent } from '../tactical-timeline.ts';
import { bombState, hudClock } from './map-hud.ts';
import { isWholeRound, roundClock, tramoEnd } from './tramo.ts';
import type { Tramo } from './tramo.ts';

/** The GIF of the Replay's tramo: its span, frame times, palette and file name. components/replay/gif-export.ts draws and encodes it. */

/** A long GIF drops frames so it stays the size a doc or a chat takes. */
export const GIF_FPS = 15;
export const GIF_MAX_FRAMES = 1800;
export const GIF_MAX_SIDE = 720;
/** The quantizer picks 255 colours; the last of the 256 slots is the transparent one. */
export const GIF_COLORS = 255;
export const GIF_TRANSPARENT = 255;
const GIF_MIN_SECONDS = 0.5;

/** A span on the transport's clock: seconds from the round's start alone, or since freeze end when superposed. */
export type GifSpan = { from: number; to: number };

/**
 * The tramo on the transport's clock. `freezeSeconds` is where the live round
 * starts on that clock (0 when superposed), `plantSeconds` the plant on it, or
 * null when the bomb was not planted. Null when there is nothing to record.
 */
export function gifSpan(options: { tramo: Tramo; freezeSeconds: number; durationSeconds: number; plantSeconds: number | null }): GifSpan | null {
  const { tramo, freezeSeconds, durationSeconds, plantSeconds } = options;
  let from = freezeSeconds;
  let to = durationSeconds;
  if (tramo.fromPlant === true) {
    if (plantSeconds === null) return null;
    from = plantSeconds;
  } else if (!isWholeRound(tramo)) {
    from = freezeSeconds + tramo.from;
    to = Math.min(durationSeconds, freezeSeconds + tramoEnd(tramo));
  }
  return to - from < GIF_MIN_SECONDS ? null : { from, to };
}

/** The frame rate for a GIF that plays this many seconds. */
export function gifFps(playSeconds: number): number {
  if (!(playSeconds > 0)) return GIF_FPS;
  return Math.max(1, Math.min(GIF_FPS, Math.floor(GIF_MAX_FRAMES / playSeconds)));
}

/** One frame: the clock it shows and how long it stays, in whole centiseconds as GIF stores them. */
export type GifFrame = { seconds: number; delayMs: number };

/**
 * The frames of a span played at `speed`. Each delay rounds to centiseconds
 * against the running total, so the GIF lasts exactly as long as the span.
 */
export function gifFrames(span: GifSpan, speed: number): GifFrame[] {
  const rate = speed > 0 ? speed : 1;
  const playSeconds = (span.to - span.from) / rate;
  const fps = gifFps(playSeconds);
  const count = Math.max(1, Math.round(playSeconds * fps));
  const frames: GifFrame[] = [];
  let shown = 0;
  for (let index = 0; index < count; index++) {
    const end = Math.round(((index + 1) * 100) / fps);
    frames.push({ seconds: span.from + (index / fps) * rate, delayMs: (end - shown) * 10 });
    shown = end;
  }
  return frames;
}

/** Every `stride`th pixel of each frame in one RGBA buffer, for the quantizer to pick the palette from. */
export function paletteSample(frames: readonly Uint8ClampedArray[], stride: number): Uint8Array {
  const step = Math.max(1, Math.floor(stride));
  let pixels = 0;
  for (const frame of frames) pixels += Math.ceil(frame.length / 4 / step);
  const out = new Uint8Array(pixels * 4);
  let at = 0;
  for (const frame of frames) {
    for (let byte = 0; byte < frame.length; byte += step * 4) {
      out[at] = frame[byte];
      out[at + 1] = frame[byte + 1];
      out[at + 2] = frame[byte + 2];
      out[at + 3] = frame[byte + 3];
      at += 4;
    }
  }
  return out;
}

/** The palette with the transparent slot at the end: GIF tables hold a power of two. */
export function gifPalette(colors: readonly (readonly number[])[]): number[][] {
  const out = colors.slice(0, GIF_COLORS).map((color) => [color[0] ?? 0, color[1] ?? 0, color[2] ?? 0]);
  while (out.length <= GIF_TRANSPARENT) out.push([0, 0, 0]);
  return out;
}

/** A pixel that did not change since the last frame becomes transparent: a still map then costs almost nothing. */
export function keepChanged(index: Uint8Array, previous: Uint8Array): Uint8Array {
  const out = new Uint8Array(index.length);
  for (let i = 0; i < index.length; i++) out[i] = index[i] === previous[i] ? GIF_TRANSPARENT : index[i];
  return out;
}

/** The GIF's size: the map as framed on screen, its longer side at most `GIF_MAX_SIDE`. */
export function gifSize(box: { width: number; height: number }): { width: number; height: number; scale: number } {
  const scale = Math.min(1, GIF_MAX_SIDE / Math.max(1, box.width, box.height));
  return { width: Math.max(1, Math.round(box.width * scale)), height: Math.max(1, Math.round(box.height * scale)), scale };
}

/**
 * The line over the GIF, as the HUD reads: the rounds and the clock since the
 * round went live, or the bomb's countdown once planted. `events` and
 * `seconds` are on the transport's clock and `freezeSeconds` is its live start.
 */
export function gifCaption(options: { rounds: string; events: readonly TimelineEvent[]; seconds: number; freezeSeconds: number }): string {
  const bomb = bombState(options.events, options.seconds);
  const clock = bomb.kind === 'planted' ? `Bomba ${Math.ceil(bomb.remaining)}` : hudClock(options.seconds - options.freezeSeconds);
  return `${options.rounds} · ${clock}`;
}

/** Where the bomb went down on the transport's clock, or null in a round without a plant. */
export function plantSecondsOf(events: readonly TimelineEvent[]): number | null {
  return events.find((entry) => entry.event.kind === TACTICAL_EVENT_KINDS.plant)?.seconds ?? null;
}

/** Characters Windows does not take in a file name. */
const RESERVED = /[\\/:*?"<>|\u0000-\u001f]+/g;

/** `Mirage FUT vs BetBoom R14 1-00 a 0-40.gif`: the tramo on the CS2 clock, as the Replay labels it. */
export function replayGifFileName(options: { map: string; teams: readonly string[]; rounds: string; span: GifSpan; freezeSeconds: number }): string {
  const clock = (seconds: number): string => roundClock(seconds - options.freezeSeconds).replace(':', '-');
  const teams = options.teams.filter((team) => team.trim() !== '').join(' vs ');
  const parts = [options.map, teams, options.rounds, `${clock(options.span.from)} a ${clock(options.span.to)}`];
  const stem = parts
    .filter((part) => part !== '')
    .join(' ')
    .replace(RESERVED, ' ')
    .replace(/\s+/g, ' ')
    .trim()
    .slice(0, 120)
    .trim();
  return `${stem === '' ? 'replay' : stem}.gif`;
}

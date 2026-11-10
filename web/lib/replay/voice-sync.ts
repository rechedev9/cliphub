/**
 * Team voice on the Replay's clock. The recording is placed by one offset:
 * demo time = audio time + offset, both in milliseconds from their own start.
 */

/** How far the audio may drift from the playhead before it is moved, in seconds. */
export const PLAYING_DRIFT_SECONDS = 0.3;
export const PAUSED_DRIFT_SECONDS = 0.05;
/** The adjust bar's drag reaches this far either side of the offset it opened with. */
export const ADJUST_RANGE_MS = 60_000;
export const NUDGE_STEPS_MS = [-1000, -100, 100, 1000] as const;

/** Browsers play audio between these rates; the Replay's speeds fit inside. */
const MIN_RATE = 0.25;
const MAX_RATE = 4;

export function demoMsOfTick(tick: number, tickrate: number): number {
  return tickrate > 0 ? (tick * 1000) / tickrate : 0;
}

/** Where the recording is at a demo tick, in seconds; null before it starts or after it ends. */
export function audioSecondsAt(at: { tick: number; tickrate: number; offsetMs: number; durationMs: number }): number | null {
  const audioMs = demoMsOfTick(at.tick, at.tickrate) - at.offsetMs;
  if (audioMs < 0 || audioMs > at.durationMs) return null;
  return audioMs / 1000;
}

/** The demo tick a moment of the recording plays at. */
export function tickOfAudioMs(audioMs: number, sync: { offsetMs: number; tickrate: number }): number {
  return Math.round(((audioMs + sync.offsetMs) * sync.tickrate) / 1000);
}

/** Whether an element at `current` seconds must jump to `target`. */
export function needsSeek(at: { current: number; target: number; playing: boolean }): boolean {
  return Math.abs(at.current - at.target) > (at.playing ? PLAYING_DRIFT_SECONDS : PAUSED_DRIFT_SECONDS);
}

/** Whether the freeze-time filter silences the voice at a tick: everything after the round went live. */
export function silentOutsideFreeze(at: { tick: number; freezeOnly: boolean; freezeEndTick: number | null }): boolean {
  return at.freezeOnly && at.freezeEndTick !== null && at.tick >= at.freezeEndTick;
}

export function playbackRate(speed: number): number {
  if (!Number.isFinite(speed) || speed <= 0) return 1;
  return Math.min(MAX_RATE, Math.max(MIN_RATE, speed));
}

/** Milliseconds as `m:ss.d`, signed when negative or when `signed` asks for a plus. */
export function formatVoiceMs(ms: number, signed = false): string {
  let sign = signed ? '+' : '';
  if (ms < 0) sign = '-';
  const tenths = Math.round(Math.abs(ms) / 100);
  const minutes = Math.floor(tenths / 600);
  const seconds = Math.floor((tenths % 600) / 10);
  return `${sign}${minutes}:${String(seconds).padStart(2, '0')},${tenths % 10}`;
}

/** The change the adjust bar would make, as the bar reads it: `Δ +1,2 s`. */
export function deltaLabel(draftMs: number, savedMs: number): string {
  const delta = Math.round((draftMs - savedMs) / 100) / 10;
  if (delta === 0) return 'Δ 0 s';
  return `Δ ${delta > 0 ? '+' : ''}${delta.toLocaleString('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 })} s`;
}

export function clampOffset(ms: number, maxMs: number): number {
  return Math.round(Math.min(maxMs, Math.max(-maxMs, ms)));
}

/** Volume of one track: the master level, silent when muted. */
export function trackVolume(level: { master: number; muted: boolean; trackMuted: boolean }): number {
  if (level.muted || level.trackMuted) return 0;
  return Math.min(1, Math.max(0, level.master));
}

import { TACTICAL_EVENT_KINDS } from './api/tactical.ts';
import type { TacticalEvent, TacticalRound } from './api/tactical.ts';
import { roundClockLabel } from './tactical-labels.ts';

/**
 * The round's clock: how a tick becomes a position on the scrub bar, and how the
 * transport advances against the wall clock.
 *
 * Playback is deliberately time-based, never frame-based. A dropped animation
 * frame makes the next `elapsedMs` bigger, and the playhead absorbs it, so the
 * replay and the timeline cannot drift apart on a busy machine the way a
 * "+1 sample per rAF" loop does.
 */

/** CS2's default tick rate, used only when a demo reports an unusable one. */
export const DEFAULT_TICKRATE = 64;

/** Playback speeds the transport offers, slowest first. */
export const TACTICAL_REPLAY_SPEEDS = [0.25, 0.5, 1, 2, 4] as const;
export type TacticalReplaySpeed = (typeof TACTICAL_REPLAY_SPEEDS)[number];

/** Seconds an arrow-key seek moves the playhead. */
export const SEEK_STEP_SECONDS = 1;

/** How far back the motion trail reaches. */
export const TRAIL_SECONDS = 2;

/** One round's playable span, in ticks and in seconds from its first tick. */
export type RoundTimeline = {
  tickrate: number;
  /** Round start: freeze time begins here, and so does the scrub bar. */
  startTick: number;
  /** Freeze time ends and the round goes live. */
  freezeEndTick: number;
  /** Round end, the last tick the bar covers. */
  endTick: number;
  /** Length of the leading freeze-time band, in seconds. */
  freezeSeconds: number;
  /** Whole bar length, in seconds. */
  durationSeconds: number;
};

function usableTickrate(tickrate: number): number {
  return Number.isFinite(tickrate) && tickrate > 0 ? tickrate : DEFAULT_TICKRATE;
}

/** The round fields the timeline is built from. */
export type TimelineRound = Pick<
  TacticalRound,
  'tick_start' | 'tick_freeze_end' | 'tick_end' | 'tick_official'
>;

/**
 * Builds a round's timeline. The end tick falls back to the official end (the
 * post-round tick) when the round's own end is missing or out of order, so a
 * demo with a ragged round transition still yields a bar with real length
 * instead of a zero-width one.
 */
export function roundTimeline(round: TimelineRound, tickrate: number): RoundTimeline {
  const rate = usableTickrate(tickrate);
  const startTick = round.tick_start;
  const candidates = [round.tick_end, round.tick_official].filter((tick) => tick > startTick);
  const endTick = candidates.length > 0 ? Math.min(...candidates) : startTick + rate;
  const freezeEndTick = Math.min(Math.max(round.tick_freeze_end, startTick), endTick);
  return {
    tickrate: rate,
    startTick,
    freezeEndTick,
    endTick,
    freezeSeconds: (freezeEndTick - startTick) / rate,
    durationSeconds: (endTick - startTick) / rate,
  };
}

/** A tick as seconds from the start of the bar. */
export function timelineSeconds(timeline: RoundTimeline, tick: number): number {
  return (tick - timeline.startTick) / timeline.tickrate;
}

/** A position on the bar as the tick it addresses. */
export function timelineTick(timeline: RoundTimeline, seconds: number): number {
  return Math.round(timeline.startTick + seconds * timeline.tickrate);
}

/** The round clock an analyst reads: seconds since the round went live, negative in freeze time. */
export function roundClockSeconds(timeline: RoundTimeline, seconds: number): number {
  return seconds - timeline.freezeSeconds;
}

/** A bar position as the round clock an analyst reads, ready to display. */
export function roundClockLabelFor(timeline: RoundTimeline, seconds: number): string {
  return roundClockLabel(roundClockSeconds(timeline, seconds));
}

/** A position on the bar as a 0..1 fraction, clamped, for laying out a marker. */
export function timelineFraction(timeline: RoundTimeline, seconds: number): number {
  if (!(timeline.durationSeconds > 0)) return 0;
  return clamp(seconds / timeline.durationSeconds, 0, 1);
}

/** Keeps a seek inside the bar. */
export function clampToTimeline(timeline: RoundTimeline, seconds: number): number {
  if (!Number.isFinite(seconds)) return 0;
  return clamp(seconds, 0, timeline.durationSeconds);
}

function clamp(value: number, min: number, max: number): number {
  if (value < min) return min;
  if (value > max) return max;
  return value;
}

/** The transport's whole mutable state: where the playhead is and whether it moves. */
export type TransportState = { position: number; playing: boolean };

/** One wall-clock step of the transport. */
export type TransportStep = {
  elapsedMs: number;
  speed: number;
  durationSeconds: number;
  loop: boolean;
};

/**
 * Advances the playhead by real elapsed time. Reaching the end either wraps
 * (loop) or stops the transport on the last frame; a non-finite or negative
 * elapsed time is treated as no time at all rather than as a jump.
 */
export function advanceTransport(state: TransportState, step: TransportStep): TransportState {
  const duration = step.durationSeconds;
  if (!(duration > 0)) return { position: 0, playing: false };
  if (!state.playing) return { position: clamp(state.position, 0, duration), playing: false };

  const elapsed = Number.isFinite(step.elapsedMs) && step.elapsedMs > 0 ? step.elapsedMs : 0;
  const speed = Number.isFinite(step.speed) && step.speed > 0 ? step.speed : 1;
  const next = clamp(state.position, 0, duration) + (elapsed / 1000) * speed;

  if (next < duration) return { position: next, playing: true };
  if (!step.loop) return { position: duration, playing: false };
  // Wrap by the whole span so a long stall (a backgrounded tab, a slow paint)
  // lands where the clock says, not one loop behind it.
  const wrapped = next % duration;
  return { position: wrapped, playing: true };
}

/** One round event placed on the bar. */
export type TimelineEvent = {
  event: TacticalEvent;
  /** Seconds from the start of the bar. */
  seconds: number;
  /** 0..1 position, for a marker's `left`. */
  fraction: number;
};

/**
 * Places a round's events on the bar, in chronological order and dropping the
 * ones outside it (a demo can carry a post-round tick), so a marker is always
 * where the scrub bar can actually be dragged.
 */
export function timelineEvents(
  timeline: RoundTimeline,
  events: readonly TacticalEvent[],
): TimelineEvent[] {
  return events
    .map((event) => ({ event, seconds: timelineSeconds(timeline, event.tick) }))
    .filter((entry) => entry.seconds >= 0 && entry.seconds <= timeline.durationSeconds)
    .sort((a, b) => a.seconds - b.seconds)
    .map((entry) => ({
      ...entry,
      fraction: timelineFraction(timeline, entry.seconds),
    }));
}

/** Whether an event is drawn as a kill marker, a bomb notch, or a utility tick. */
export function isBombEvent(event: TacticalEvent): boolean {
  return (
    event.kind === TACTICAL_EVENT_KINDS.plant ||
    event.kind === TACTICAL_EVENT_KINDS.defuse ||
    event.kind === TACTICAL_EVENT_KINDS.explode
  );
}

/** The events already in the past at a playhead position. */
export function elapsedEvents(entries: readonly TimelineEvent[], seconds: number): TimelineEvent[] {
  return entries.filter((entry) => entry.seconds <= seconds);
}

/** Seek epsilon: a step must clear the current event, not land back on it. */
const EVENT_SEEK_EPSILON = 1e-3;

/**
 * The position of the next (or previous) event, for the shift-arrow seek.
 * Returns undefined at the ends so the caller can leave the playhead alone
 * instead of silently jumping to a boundary.
 */
export function seekEventSeconds(
  entries: readonly TimelineEvent[],
  seconds: number,
  direction: 1 | -1,
): number | undefined {
  if (direction === 1) {
    return entries.find((entry) => entry.seconds > seconds + EVENT_SEEK_EPSILON)?.seconds;
  }
  for (let i = entries.length - 1; i >= 0; i -= 1) {
    if (entries[i].seconds < seconds - EVENT_SEEK_EPSILON) return entries[i].seconds;
  }
  return undefined;
}

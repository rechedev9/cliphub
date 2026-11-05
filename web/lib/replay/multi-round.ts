import type { RoundTimeline, TimelineEvent } from '../tactical-timeline.ts';

/**
 * The shared clock of superposed rounds: seconds since each round's freeze
 * end, so the same second of every round plays at once.
 */

/** The longest live span among the rounds, at least one second so the bar has length. */
export function liveDuration(timelines: readonly RoundTimeline[]): number {
  let longest = 0;
  for (const timeline of timelines) longest = Math.max(longest, timeline.durationSeconds - timeline.freezeSeconds);
  return Math.max(1, longest);
}

/** The tick a round is at, `seconds` after its freeze end; a shorter round holds its last tick. */
export function alignedTick(timeline: RoundTimeline, seconds: number): number {
  const tick = Math.round(timeline.freezeEndTick + Math.max(0, seconds) * timeline.tickrate);
  return Math.min(timeline.endTick, Math.max(timeline.startTick, tick));
}

/** A round-start position as live seconds, for switching the transport into the shared clock. */
export function toLiveSeconds(timeline: RoundTimeline, seconds: number): number {
  return Math.max(0, seconds - timeline.freezeSeconds);
}

/** A live position back on the round's own bar. */
export function fromLiveSeconds(timeline: RoundTimeline, seconds: number): number {
  return Math.min(timeline.durationSeconds, seconds + timeline.freezeSeconds);
}

/** The bar the transport draws for the shared clock: no freeze band, as long as the longest round. */
export function liveTimeline(timelines: readonly RoundTimeline[], tickrate: number): RoundTimeline {
  const duration = liveDuration(timelines);
  return {
    tickrate,
    startTick: 0,
    freezeEndTick: 0,
    endTick: Math.round(duration * tickrate),
    freezeSeconds: 0,
    durationSeconds: duration,
  };
}

/** The focused round's events moved onto the shared clock; freeze-time events are dropped. */
export function liveEvents(timeline: RoundTimeline, events: readonly TimelineEvent[], duration: number): TimelineEvent[] {
  const out: TimelineEvent[] = [];
  for (const entry of events) {
    const seconds = entry.seconds - timeline.freezeSeconds;
    if (seconds < 0 || seconds > duration) continue;
    out.push({ event: entry.event, seconds, fraction: duration > 0 ? seconds / duration : 0 });
  }
  return out;
}

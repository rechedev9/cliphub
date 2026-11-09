import { advanceTransport } from '../tactical-timeline.ts';

/** The scrubber's step; a drag to the edge can land up to one step short of the end. */
export const SCRUB_STEP_SECONDS = 0.05;

/** One animation frame of a playing replay. */
export type PlaybackFrame = {
  position: number;
  elapsedMs: number;
  speed: number;
  durationSeconds: number;
  loop: boolean;
  /** The scrubber is held, so the hand owns the playhead. */
  scrubbing: boolean;
  /** The user moved the playhead since the last frame that played. */
  seeked: boolean;
  /** A round ending here hands over to the next one: a playlist with more rounds. */
  hasNext: boolean;
};

export type PlaybackAction = 'play' | 'hold' | 'next' | 'stop';
export type PlaybackStep = { position: number; action: PlaybackAction };

/** Advances one frame. Only a round played into its end moves on; one seeked to its end stops there,
 * so a scrubber held at the edge cannot run through every round of the playlist. */
export function playbackStep(frame: PlaybackFrame): PlaybackStep {
  if (frame.scrubbing) return { position: frame.position, action: 'hold' };
  if (frame.seeked && !frame.loop && atEnd(frame.position, frame.durationSeconds)) {
    return { position: frame.position, action: 'stop' };
  }
  const next = advanceTransport(
    { position: frame.position, playing: true },
    { elapsedMs: frame.elapsedMs, speed: frame.speed, durationSeconds: frame.durationSeconds, loop: frame.loop },
  );
  if (next.playing) return { position: next.position, action: 'play' };
  return { position: next.position, action: frame.hasNext && !frame.seeked ? 'next' : 'stop' };
}

/** Play from the start when the playhead sits at the end, within a scrubber step. */
export function playStartSeconds(position: number, durationSeconds: number): number {
  return atEnd(position, durationSeconds) ? 0 : position;
}

function atEnd(position: number, durationSeconds: number): boolean {
  return position >= durationSeconds - SCRUB_STEP_SECONDS;
}

/** A playing position kept inside a span: past its end, or before its start, it starts over. */
export function loopWithin(position: number, span: { from: number; to: number }): number {
  return position < span.from || position >= span.to ? span.from : position;
}

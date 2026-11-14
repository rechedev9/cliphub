import type { ReplayClock } from './clock.ts';

export type ClockThrottleOptions = {
  /** How many times a second a moving playhead is passed on. */
  stepsPerSecond: number;
  /** How long the clock must rest before its exact second is passed on. */
  settleMs: number;
  onSeconds: (seconds: number) => void;
};

/**
 * Follows the clock for text that shows the playhead: a few exact updates a second while it moves, and the
 * exact second once it rests, so a paused or seeked replay never shows a moment the map has already left.
 */
export function throttleClock(clock: ReplayClock, { stepsPerSecond, settleMs, onSeconds }: ClockThrottleOptions): () => void {
  let heard = clock.seconds;
  let settle: ReturnType<typeof setTimeout> | undefined;
  const stepOf = (seconds: number): number => Math.floor(seconds * stepsPerSecond);
  const emit = (seconds: number): void => {
    heard = seconds;
    onSeconds(seconds);
  };
  onSeconds(heard);
  const unsubscribe = clock.subscribe((seconds) => {
    clearTimeout(settle);
    settle = undefined;
    if (stepOf(seconds) !== stepOf(heard)) {
      emit(seconds);
      return;
    }
    if (seconds !== heard) settle = setTimeout(() => emit(seconds), settleMs);
  });
  return () => {
    clearTimeout(settle);
    unsubscribe();
  };
}

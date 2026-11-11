'use client';

import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useTramo } from '@/components/replay/tramo-bar';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { ROUND_CLOCK_SECONDS, TRAMO_MIN_SECONDS, isWholeRound, tramoEnd } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { LIVE_SAMPLE_SECONDS } from '@/lib/heat-live';

/** The owner keeps the tramo in the URL; it hears of a drag this long after the last move. */
const PUSH_DELAY_MS = 250;

export const HEAT_PLAY_SPEED = 2;

export type HeatPlayback = {
  /** The tramo the map paints now: it moves with every drag and every frame of playback. */
  tramo: Tramo;
  setTramo: (next: Tramo) => void;
  playing: boolean;
  toggle: () => void;
  /** Playback runs on the shared round clock, which "Desde el plant" does not have. */
  canPlay: boolean;
  speed: number;
  setSpeed: (speed: number) => void;
  /** The second the players are drawn at, or null when the tramo has no single end to show. */
  nowRef: RefObject<number | null>;
};

/** Where the players stand for a tramo at rest: its end, unless it runs the whole round or from the plant. */
export function playersSecond(tramo: Tramo): number | null {
  return tramo.fromPlant === true || isWholeRound(tramo) ? null : tramoEnd(tramo);
}

function sameTramo(a: Tramo, b: Tramo): boolean {
  return a.from === b.from && a.to === b.to && a.fromPlant === b.fromPlant;
}

/** Whole seconds and the minimum width, as the URL keeps a tramo. */
function settled(tramo: Tramo): Tramo {
  if (tramo.fromPlant === true) return tramo;
  return { from: Math.round(tramo.from), to: Math.max(Math.round(tramoEnd(tramo)), Math.round(tramo.from) + TRAMO_MIN_SECONDS) };
}

/**
 * The heatmap's own copy of the tramo, so a drag or playback repaints at once while
 * the owner's URL catches up, and a playback that grows the tramo's end second by second.
 * `onFrame` repaints the players between the tramo's quarter-second steps.
 */
export function useHeatPlayback({ control, onFrame }: { control: TramoControl | undefined; onFrame: () => void }): HeatPlayback {
  const owner = useTramo(control);
  const [tramo, setLive] = useState<Tramo>(owner.tramo);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(HEAT_PLAY_SPEED);
  const pushed = useRef(owner.tramo);
  const pushTimer = useRef<number | undefined>(undefined);
  const nowRef = useRef<number | null>(playersSecond(owner.tramo));
  const ownerChange = owner.onChange;

  useEffect(() => {
    if (sameTramo(owner.tramo, pushed.current)) return;
    pushed.current = owner.tramo;
    setPlaying(false);
    setLive(owner.tramo);
  }, [owner.tramo]);

  useEffect(() => () => window.clearTimeout(pushTimer.current), []);

  const push = useCallback(
    (next: Tramo, delay: number) => {
      window.clearTimeout(pushTimer.current);
      pushTimer.current = window.setTimeout(() => {
        pushed.current = next;
        ownerChange(next);
      }, delay);
    },
    [ownerChange],
  );

  const setTramo = useCallback(
    (next: Tramo) => {
      setPlaying(false);
      setLive(next);
      push(next, PUSH_DELAY_MS);
    },
    [push],
  );

  useEffect(() => {
    if (!playing) nowRef.current = playersSecond(tramo);
  }, [playing, tramo]);

  const latest = useRef(tramo);
  useEffect(() => {
    latest.current = tramo;
  }, [tramo]);

  // A stop leaves the tramo where playback reached, in whole seconds, and tells the owner.
  const stop = useCallback(
    (reached: Tramo) => {
      setPlaying(false);
      const rest = settled(reached);
      setLive(rest);
      push(rest, 0);
    },
    [push],
  );

  useEffect(() => {
    if (!playing) return;
    const from = latest.current.from;
    let at = nowRef.current ?? from;
    let last = performance.now();
    let frame = 0;
    const loop = (time: number): void => {
      at = Math.min(ROUND_CLOCK_SECONDS, at + ((time - last) / 1000) * speed);
      last = time;
      nowRef.current = at;
      const end = Math.max(from + LIVE_SAMPLE_SECONDS, Math.floor(at / LIVE_SAMPLE_SECONDS) * LIVE_SAMPLE_SECONDS);
      setLive((current) => (current.to === end && current.from === from ? current : { from, to: end }));
      onFrame();
      if (at >= ROUND_CLOCK_SECONDS) {
        stop({ from, to: ROUND_CLOCK_SECONDS });
        return;
      }
      frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(frame);
  }, [playing, speed, onFrame, stop]);

  const canPlay = tramo.fromPlant !== true;
  const toggle = useCallback(() => {
    if (playing) {
      stop(latest.current);
      return;
    }
    if (!canPlay) return;
    // From a tramo that already reaches the end, or the whole round, playback starts over at its start.
    const end = playersSecond(tramo);
    if (end === null || end >= ROUND_CLOCK_SECONDS) {
      nowRef.current = tramo.from;
      setLive({ from: tramo.from, to: tramo.from + LIVE_SAMPLE_SECONDS });
    } else nowRef.current = end;
    setPlaying(true);
  }, [canPlay, playing, stop, tramo]);

  return { tramo, setTramo, playing, toggle, canPlay, speed, setSpeed, nowRef };
}

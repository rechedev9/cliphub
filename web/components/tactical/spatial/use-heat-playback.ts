'use client';

import { useCallback, useEffect, useRef, useState, type RefObject } from 'react';
import { useTramo } from '@/components/replay/tramo-bar';
import type { TramoControl } from '@/components/replay/tramo-bar';
import { ROUND_CLOCK_SECONDS, TRAMO_MIN_SECONDS, isWholeRound, tramoEnd } from '@/lib/replay/tramo';
import type { Tramo } from '@/lib/replay/tramo';
import { LIVE_SAMPLE_SECONDS } from '@/lib/heat-live';

/** The owner keeps the tramo in the URL; between drag moves it hears of the tramo this long after the last one. */
const PUSH_DELAY_MS = 250;
/** Playback hands the reached tramo over at most this often, so leaving mid-play keeps it. */
const PLAY_PUSH_MS = 1000;

export const HEAT_PLAY_SPEED = 2;

export type HeatPlayback = {
  /** The tramo the map paints now: it moves with every drag and every frame of playback. */
  tramo: Tramo;
  setTramo: (next: Tramo) => void;
  /** Hands a tramo still waiting for the owner over at once, as a drag or a key is released. */
  flush: () => void;
  playing: boolean;
  toggle: () => void;
  /** Playback runs on the shared round clock, which "Desde el plant" does not have. */
  canPlay: boolean;
  speed: number;
  setSpeed: (speed: number) => void;
  /** The second the players stand at while playback runs. */
  nowRef: RefObject<number | null>;
};

/** Where the players stand for a tramo at rest: its end, unless it runs the whole round or from the plant. */
export function playersSecond(tramo: Tramo): number | null {
  return tramo.fromPlant === true || isWholeRound(tramo) ? null : tramoEnd(tramo);
}

function sameTramo(a: Tramo, b: Tramo): boolean {
  return a.from === b.from && a.to === b.to && a.fromPlant === b.fromPlant;
}

/** Whole seconds and the minimum width, as the URL keeps a tramo; an open end stays open. */
function settled(tramo: Tramo): Tramo {
  if (tramo.fromPlant === true) return tramo;
  const from = Math.round(tramo.from);
  if (tramo.to <= 0) return { from, to: 0 };
  return { from, to: Math.max(Math.round(tramoEnd(tramo)), from + TRAMO_MIN_SECONDS) };
}

/**
 * The heatmap's own tramo, repainted at once while the owner's URL catches up, and a playback that
 * grows its end until the longest round (`duration`, live seconds) is over.
 */
export function useHeatPlayback(options: { control: TramoControl | undefined; onFrame: () => void; duration: number }): HeatPlayback {
  const { control, onFrame, duration } = options;
  const owner = useTramo(control);
  const [tramo, setLive] = useState<Tramo>(owner.tramo);
  const [playing, setPlaying] = useState(false);
  const [speed, setSpeed] = useState(HEAT_PLAY_SPEED);
  const pushed = useRef(owner.tramo);
  const pending = useRef<Tramo | null>(null);
  const pushTimer = useRef<number | undefined>(undefined);
  const nowRef = useRef<number | null>(null);
  // The owner writes the tramo over its current URL, so a late push must use its latest writer.
  const ownerChange = useRef(owner.onChange);
  const durationRef = useRef(duration);
  const latest = useRef(tramo);

  useEffect(() => {
    ownerChange.current = owner.onChange;
    durationRef.current = duration;
    latest.current = tramo;
  });

  useEffect(() => {
    if (sameTramo(owner.tramo, pushed.current)) return;
    pushed.current = owner.tramo;
    pending.current = null;
    setPlaying(false);
    setLive(owner.tramo);
  }, [owner.tramo]);

  useEffect(() => () => window.clearTimeout(pushTimer.current), []);

  const send = useCallback(() => {
    window.clearTimeout(pushTimer.current);
    pushTimer.current = undefined;
    const next = pending.current;
    if (next === null) return;
    pending.current = null;
    pushed.current = next;
    ownerChange.current(next);
  }, []);

  const push = useCallback(
    (next: Tramo, delay: number) => {
      pending.current = next;
      window.clearTimeout(pushTimer.current);
      pushTimer.current = window.setTimeout(send, delay);
    },
    [send],
  );

  // Unlike `push`, a newer tramo does not move the timer, so a steady playback still reaches the owner.
  const pushSoon = useCallback(
    (next: Tramo) => {
      pending.current = next;
      if (pushTimer.current === undefined) pushTimer.current = window.setTimeout(send, PLAY_PUSH_MS);
    },
    [send],
  );

  const setTramo = useCallback(
    (next: Tramo) => {
      setPlaying(false);
      setLive(next);
      push(next, PUSH_DELAY_MS);
    },
    [push],
  );

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
      const limit = Math.max(ROUND_CLOCK_SECONDS, durationRef.current);
      at = Math.min(limit, at + ((time - last) / 1000) * speed);
      last = time;
      nowRef.current = at;
      // Past 1:55 only planted rounds go on, and the strip cannot show it: the tramo runs open to the round end.
      const end = at > ROUND_CLOCK_SECONDS ? 0 : Math.max(from + LIVE_SAMPLE_SECONDS, Math.floor(at / LIVE_SAMPLE_SECONDS) * LIVE_SAMPLE_SECONDS);
      setLive((current) => (current.to === end && current.from === from ? current : { from, to: end }));
      pushSoon(settled({ from, to: end }));
      onFrame();
      if (at >= limit) {
        stop({ from, to: 0 });
        return;
      }
      frame = requestAnimationFrame(loop);
    };
    frame = requestAnimationFrame(loop);
    return () => cancelAnimationFrame(frame);
  }, [playing, speed, onFrame, stop, pushSoon]);

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

  return { tramo, setTramo, flush: send, playing, toggle, canPlay, speed, setSpeed, nowRef };
}

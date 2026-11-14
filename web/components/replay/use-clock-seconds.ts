'use client';

import { useCallback, useEffect, useState } from 'react';
import type { CentreRound } from '@/lib/replay/centre-round';
import type { ReplayClock } from '@/lib/replay/clock';
import { throttleClock } from '@/lib/replay/clock-throttle';

const UI_TICKS_PER_SECOND = 4;
const SETTLE_MS = 80;

/** The replay clock and how a round reads a tick off it. */
export type RoundPlayhead = {
  clock: ReplayClock;
  tickOf: (round: CentreRound, seconds: number) => number;
};

/**
 * A round's tick at the playhead, for a panel that shows it in text. It re-renders a few times a second while
 * the clock moves, so playback does not re-render the whole workspace.
 */
export function usePlayheadTick({ clock, tickOf }: RoundPlayhead): (round: CentreRound) => number {
  const [seconds, setSeconds] = useState(() => clock.seconds);
  useEffect(() => throttleClock(clock, { stepsPerSecond: UI_TICKS_PER_SECOND, settleMs: SETTLE_MS, onSeconds: setSeconds }), [clock]);
  return useCallback((round: CentreRound) => tickOf(round, seconds), [seconds, tickOf]);
}

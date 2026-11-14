'use client';

import { useEffect, useState } from 'react';
import type { ReplayClock } from '@/lib/replay/clock';
import { throttleClock } from '@/lib/replay/clock-throttle';

/** How often a panel that reads the playhead re-renders; the map and the scrubber follow every frame on their own. */
const UI_TICKS_PER_SECOND = 4;
const SETTLE_MS = 80;

/**
 * The playhead for a component that shows it in text, a few times a second while it moves. Kept out of the
 * workspace so playback does not re-render the round strip, the transport and every panel with it.
 */
export function useClockSeconds(clock: ReplayClock): number {
  const [seconds, setSeconds] = useState(() => clock.seconds);
  useEffect(() => throttleClock(clock, { stepsPerSecond: UI_TICKS_PER_SECOND, settleMs: SETTLE_MS, onSeconds: setSeconds }), [clock]);
  return seconds;
}

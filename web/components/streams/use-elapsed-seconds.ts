'use client';

import { useEffect, useState } from 'react';

/**
 * Whole seconds since `active` last became true, or 0 while it is false.
 *
 * The kit's `LongOperation` deliberately has no internal timer — the caller
 * owns the clock — and the stream render needs one. The interval only exists
 * while the operation runs, so an idle editor schedules nothing.
 */
export function useElapsedSeconds(active: boolean): number {
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (!active) {
      setElapsed(0);
      return;
    }
    const started = Date.now();
    setElapsed(0);
    const timer = setInterval(() => setElapsed(Math.floor((Date.now() - started) / 1000)), 1000);
    return () => clearInterval(timer);
  }, [active]);

  return elapsed;
}

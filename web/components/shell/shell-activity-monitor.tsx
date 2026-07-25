'use client';

import { useEffect } from 'react';
import { api } from '@/lib/api';
import { startPollLoop } from '@/lib/poll-loop';
import {
  CAPTURE_ACTIVE_ATTRIBUTE,
  publishShellActivity,
  resetShellActivity,
  shellActivityIsStale,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';

// Matches the Library's own cadence so the two never drift out of step, and
// backs off hard once nothing is in flight — a shell-wide poll is not worth a
// request every 1.5s to say "still idle".
const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

/**
 * Keeps the shell's job state alive on every route and mirrors it onto <html>.
 *
 * `data-capture-active` is what gates the whole depth/animation ladder in
 * globals.css: while a reel is recording or composing, Studio shares the GPU
 * with cs2.exe and HLAE, which is a harder constraint than the efficiency
 * profile. Nothing set the attribute before this component, so that gate was
 * inert.
 *
 * The loop defers to any page already polling `api.listVideos()` — see
 * `lib/shell-activity.ts` for how a page pushes — so mounting this alongside
 * the Library costs zero extra requests.
 */
export function ShellActivityMonitor(): null {
  useEffect(() => {
    const root = document.documentElement;

    const mirror = (): void => {
      if (shellActivitySnapshot().capturing) {
        root.setAttribute(CAPTURE_ACTIVE_ATTRIBUTE, 'true');
      } else {
        root.removeAttribute(CAPTURE_ACTIVE_ATTRIBUTE);
      }
    };

    const unsubscribe = subscribeToShellActivity(mirror);
    mirror();

    const stop = startPollLoop({
      tick: async () => {
        if (shellActivityIsStale(Date.now())) {
          publishShellActivity(await api.listVideos(), Date.now());
        }
        return shellActivitySnapshot().jobs.length > 0 ? 'fast' : 'idle';
      },
      fastMs: FAST_POLL_MS,
      idleMs: IDLE_POLL_MS,
    });

    return () => {
      stop();
      unsubscribe();
      resetShellActivity();
      root.removeAttribute(CAPTURE_ACTIVE_ATTRIBUTE);
    };
  }, []);

  return null;
}

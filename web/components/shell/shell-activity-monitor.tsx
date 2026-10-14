'use client';

import { useEffect } from 'react';
import { api } from '@/lib/api';
import { cloudApi } from '@/lib/api/cloud';
import { streamsApi } from '@/lib/api/streams';
import { cloudAccountSnapshot } from '@/lib/cloud/account-store';
import { cloudDeviceLinked } from '@/lib/cloud/account-view';
import { startPollLoop } from '@/lib/poll-loop';
import {
  CAPTURE_ACTIVE_ATTRIBUTE,
  collectShellJobs,
  publishShellJobs,
  resetShellActivity,
  shellActivityIsStale,
  shellActivitySnapshot,
  subscribeToShellActivity,
} from '@/lib/shell-activity';

// Matches the Library's own cadence so the two never drift out of step, and
// backs off hard once nothing is in flight.
const FAST_POLL_MS = 1500;
const IDLE_POLL_MS = 10000;

function settled<T>(result: PromiseSettledResult<T[]>): T[] {
  return result.status === 'fulfilled' ? result.value : [];
}

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
          const [videos, matches, streams, cloud] = await Promise.allSettled([
            api.listVideos(),
            api.listMatches(),
            streamsApi.listJobs(),
            cloudApi.jobs(),
          ]);
          if (
            videos.status === 'rejected' &&
            matches.status === 'rejected' &&
            streams.status === 'rejected'
          ) {
            return 'fast';
          }
          publishShellJobs(
            collectShellJobs({
              videos: settled(videos),
              matches: settled(matches),
              streams: settled(streams),
              cloud: settled(cloud),
              cloudLinked: cloudDeviceLinked(cloudAccountSnapshot()),
            }),
            Date.now(),
          );
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

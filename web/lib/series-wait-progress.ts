import type { JobProgress, SeriesDemo } from './api/types.ts';
import type { SeriesGroup } from './series-grouping.ts';
import { seriesStatusIsPending } from './series-status.ts';

/** Snapshot the series LongOperation prints: a live worker poll, else maps done. */
export function seriesWaitProgress(
  groups: readonly SeriesGroup<SeriesDemo>[],
  list: readonly SeriesDemo[],
): JobProgress {
  const pending = list.filter((demo) => seriesStatusIsPending(demo.status));
  const working = pending.find((demo) => demo.progress);
  if (working?.progress) return working.progress;
  const doneMaps = groups.filter((group) => !group.demos.some((demo) => seriesStatusIsPending(demo.status))).length;
  return { done: doneMaps, total: groups.length, unit: 'maps', label: 'mapas' };
}

import { seriesStatusIsForgeable, seriesStatusIsPending } from './series-status.ts';

/** File-name tokens used to group HLTV `-pN` parts of one map. */
export type ParsedSeriesFileName = { base: string; mapOrder: number | null; part: number | null };

/** Map-level status: forgeable wins, then pending, then failed. */
export function representativeSeriesStatus(statuses: readonly string[]): string {
  const forgeable = statuses.find(seriesStatusIsForgeable);
  if (forgeable !== undefined) return forgeable;
  const pending = statuses.find(seriesStatusIsPending);
  if (pending !== undefined) return pending;
  const failed = statuses.find((status) => status === 'failed');
  if (failed !== undefined) return failed;
  return statuses[0] ?? 'scanned';
}

/** A logical map card: one map's parts, sorted, plus its series-ordering key. */
export type SeriesGroup<T> = { key: string; mapOrder: number | null; demos: T[] };

/** Trailing `.dem` extension; optional, so a name without it is handled too. */
const DEM_EXTENSION_RE = /\.dem(?:\.zst)?$/i;
/** The `-p<n>` part suffix at the very end of the extension-less name. */
const PART_SUFFIX_RE = /-p(\d+)$/i;
/** The `m<n>` map-number token, delimited by dashes or the string bounds. */
const MAP_ORDER_RE = /(?:^|-)m(\d+)(?:-|$)/i;

/** Parse base / map order / part from a series file name. */
export function parseSeriesFileName(fileName: string): ParsedSeriesFileName {
  const nameNoExt = fileName.replace(DEM_EXTENSION_RE, '');
  const partMatch = PART_SUFFIX_RE.exec(nameNoExt);
  const part = partMatch ? Number.parseInt(partMatch[1], 10) : null;
  const base = partMatch ? nameNoExt.slice(0, partMatch.index) : nameNoExt;
  const mapMatch = MAP_ORDER_RE.exec(base);
  const mapOrder = mapMatch ? Number.parseInt(mapMatch[1], 10) : null;
  return { base, mapOrder, part };
}

/** A part-suffixed demo's part number for member ordering; 0 when absent. */
function partNumberOf(fileName: string | undefined): number {
  if (fileName === undefined) return 0;
  return parseSeriesFileName(fileName).part ?? 0;
}

/** Fold `-pN` parts that share a base into one map group. */
export function groupSeriesDemos<T extends { fileName?: string; jobId?: string }>(demos: readonly T[]): Array<SeriesGroup<T>> {
  const groups: Array<SeriesGroup<T>> = [];
  const partGroupByBase = new Map<string, SeriesGroup<T>>();
  const seenJobs = new Set<string>();

  demos.forEach((demo, index) => {
    if (demo.jobId !== undefined) {
      if (seenJobs.has(demo.jobId)) return;
      seenJobs.add(demo.jobId);
    }
    const parsed = demo.fileName !== undefined ? parseSeriesFileName(demo.fileName) : null;
    if (parsed !== null && parsed.part !== null) {
      const key = parsed.base.toLowerCase();
      const existing = partGroupByBase.get(key);
      if (existing) {
        existing.demos.push(demo);
      } else {
        const group: SeriesGroup<T> = { key, mapOrder: parsed.mapOrder, demos: [demo] };
        partGroupByBase.set(key, group);
        groups.push(group);
      }
    } else {
      // Singleton: a unique key keeps two extensionless/part-less demos apart.
      groups.push({ key: `#${index}`, mapOrder: parsed?.mapOrder ?? null, demos: [demo] });
    }
  });

  for (const group of groups) {
    if (group.demos.length > 1) {
      group.demos.sort((a, b) => partNumberOf(a.fileName) - partNumberOf(b.fileName));
    }
  }

  const everyHasMapOrder = groups.every((group) => group.mapOrder !== null);
  if (everyHasMapOrder) {
    // `?? 0` never fires here (every mapOrder is non-null); it only spares a cast.
    return [...groups].sort((a, b) => (a.mapOrder ?? 0) - (b.mapOrder ?? 0));
  }
  return groups;
}

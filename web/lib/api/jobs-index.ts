// Pure Partidas index: /api/demos/jobs → Matches/series. Status filters stay here.

import type { DemoPlayer, Match, MatchStats } from './types.ts';
import { PLAN_READY_STATUSES } from './types.ts';
import { prettifyMap } from './map.ts';
import { groupSeriesDemos } from '../series-grouping.ts';

/** One /api/demos/jobs row: listing fields only, no kill plan. */
export type IndexedJob = {
  jobId: string;
  status: string;
  failureReason?: string;
  fileName?: string;
  seriesId?: string;
  targetSteamId?: string;
  /** ISO-8601 upload time; the Partidas list sorts newest first by it. */
  createdAt?: string;
};

/** Statuses with a roster scan. Shared with getSeries. */
export const ROSTER_READY: ReadonlySet<string> = new Set([
  'scanned',
  'parsing',
  'parsed',
  'recording',
  'recorded',
  'composing',
  'composed',
  'done',
]);

/** True once a demo has a roster scan, so it belongs in the Partidas list. */
export function jobHasRoster(status: string): boolean {
  return ROSTER_READY.has(status);
}

/** Epoch ms for a job's upload time; 0 (sorts last) when absent or unparseable. */
export function jobCreatedAtMs(job: IndexedJob): number {
  if (!job.createdAt) return 0;
  const ms = Date.parse(job.createdAt);
  return Number.isNaN(ms) ? 0 : ms;
}

/** Roster-ready jobs for Partidas, newest first. Failed jobs never list. */
export function listableJobs(jobs: readonly IndexedJob[]): IndexedJob[] {
  return jobs
    .filter((job) => jobHasRoster(job.status))
    .sort((a, b) => jobCreatedAtMs(b) - jobCreatedAtMs(a));
}

function jobHasPlan(status: string): boolean {
  return PLAN_READY_STATUSES.has(status);
}

export function planReadyJobs(jobs: readonly IndexedJob[]): IndexedJob[] {
  return jobs
    .filter((job) => jobHasPlan(job.status))
    .sort((a, b) => jobCreatedAtMs(b) - jobCreatedAtMs(a));
}

/** One uploaded series, as the Partidas SERIES section lists it. */
export type SeriesSummary = { seriesId: string; mapCount: number; createdAt: number };

/** One summary per series_id, newest first. Split demo parts count as one map. */
export function summarizeSeries(jobs: readonly IndexedJob[]): SeriesSummary[] {
  const bySeries = new Map<string, IndexedJob[]>();
  for (const job of jobs) {
    if (!job.seriesId) continue;
    const existing = bySeries.get(job.seriesId);
    if (existing) existing.push(job);
    else bySeries.set(job.seriesId, [job]);
  }
  return Array.from(bySeries, ([seriesId, seriesJobs]) => {
    // The series started with its earliest demo; jobs without a time count as 0.
    const times = seriesJobs.map(jobCreatedAtMs).filter((at) => at > 0);
    return {
      seriesId,
      mapCount: groupSeriesDemos(seriesJobs).length,
      createdAt: times.length > 0 ? Math.min(...times) : 0,
    };
  }).sort((a, b) => b.createdAt - a.createdAt);
}

/** Zeroed scoreboard for a listed upload whose roster could not be read. */
export const ZERO_STATS: MatchStats = { kills: 0, deaths: 0, assists: 0, mvps: 0, kd: 0 };

/** Target scoreboard as Match stats. K/D is kills when deaths is 0. */
export function statsFromPlayer(player: DemoPlayer): MatchStats {
  const { kills, deaths, assists } = player;
  return {
    kills,
    deaths,
    assists,
    mvps: player.mvps,
    kd: deaths ? Number((kills / deaths).toFixed(2)) : kills,
    rating: player.rating,
    adr: player.adr,
    kast: player.kast,
    hsPct: player.hsPct,
  };
}

/** A listed job's headline when its roster has no map yet: its file name, else a placeholder. */
function jobHeadline(job: IndexedJob): string {
  return job.fileName ?? 'Partida';
}

/** Indexed job → listed Match. Missing roster still lists a zeroed filename row. */
export function jobToMatch(job: IndexedJob, enrichment?: { map?: string; player?: DemoPlayer }): Match {
  const rawMap = enrichment?.map;
  const player = enrichment?.player;
  const match: Match = {
    id: job.jobId,
    map: rawMap ? prettifyMap(rawMap) : jobHeadline(job),
    score: '',
    playedAt: job.createdAt ?? new Date(jobCreatedAtMs(job)).toISOString(),
    stats: player ? statsFromPlayer(player) : { ...ZERO_STATS },
    decentPlays: 0,
    source: 'upload',
  };
  // Name the row after the clipped/target player when the roster resolved them;
  // leave it off (no stray separator) for an unenriched or nameless entry.
  if (player?.name) match.player = player.name;
  return match;
}

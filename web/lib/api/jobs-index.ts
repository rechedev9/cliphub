// Pure Partidas index: /api/demos/jobs → Matches/series. Status filters stay here.

import type { DemoPlayer, Match, MatchStats } from './types.ts';
import { PLAN_READY_STATUSES, ROSTER_READY_STATUSES } from './types.ts';
import { prettifyMap } from './map.ts';
import { groupSeriesDemos } from '../series-grouping.ts';

/** Roster summary the orchestrator inlines per listed job (server field names). */
export type IndexedJobSummary = {
  match?: { map?: string; score_ct?: number; score_t?: number; rounds?: number };
  target?: {
    steamid64: string;
    name: string;
    team?: string;
    kills?: number;
    deaths?: number;
    assists?: number;
    headshots?: number;
    mvps?: number;
    rounds?: number;
    adr?: number;
    hs_pct?: number;
    kast?: number;
    rating?: number;
  };
};

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
  /** Present once the roster scan finished; saves the per-job roster request. */
  summary?: IndexedJobSummary;
};

/** The list enrichment carried inline by the job row, or null when the roster must still be fetched. */
export function enrichmentFromSummary(job: IndexedJob): { map?: string; player?: DemoPlayer } | null {
  const summary = job.summary;
  if (summary === undefined) return null;
  const out: { map?: string; player?: DemoPlayer } = {};
  if (summary.match?.map) out.map = summary.match.map;
  const target = summary.target;
  if (target !== undefined) {
    out.player = {
      steamId: target.steamid64,
      name: target.name,
      team: target.team === 'CT' || target.team === 'T' ? target.team : '',
      kills: target.kills ?? 0,
      deaths: target.deaths ?? 0,
      assists: target.assists ?? 0,
      headshots: target.headshots ?? 0,
      mvps: target.mvps ?? 0,
      rounds: target.rounds ?? 0,
      adr: target.adr ?? 0,
      hsPct: target.hs_pct ?? 0,
      kast: target.kast ?? 0,
      rating: target.rating ?? 0,
    };
  }
  return out;
}

/** True once a demo has a roster scan, so it belongs in the Partidas list. */
export function jobHasRoster(status: string): boolean {
  return ROSTER_READY_STATUSES.has(status);
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
    status: job.status,
  };
  // Name the row after the clipped/target player when the roster resolved them;
  // leave it off (no stray separator) for an unenriched or nameless entry.
  if (player?.name) match.player = player.name;
  return match;
}

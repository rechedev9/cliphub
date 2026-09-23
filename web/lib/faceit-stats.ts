import type { FaceitMatch, FaceitMatchStats } from './api/faceit.ts';

type FaceitPerformance = {
  wins: number;
  losses: number;
  unknown: number;
  winRate: number | undefined;
  kd: number | undefined;
  adr: number | undefined;
  headshots: number | undefined;
};

function average(values: (number | undefined)[]): number | undefined {
  const known = values.filter((value): value is number => value !== undefined && Number.isFinite(value));
  return known.length === 0 ? undefined : known.reduce((sum, value) => sum + value, 0) / known.length;
}

export function summarizeFaceitMatches(matches: FaceitMatch[]): FaceitPerformance {
  const wins = matches.filter((match) => match.stats?.result === 'win').length;
  const losses = matches.filter((match) => match.stats?.result === 'loss').length;
  const decided = wins + losses;
  return {
    wins,
    losses,
    unknown: matches.length - decided,
    winRate: decided === 0 ? undefined : Math.round((wins / decided) * 100),
    kd: average(matches.map((match) => match.stats?.kd_ratio)),
    adr: average(matches.map((match) => match.stats?.adr)),
    headshots: average(matches.map((match) => match.stats?.headshots_percent)),
  };
}

export type FaceitTrend = {
  /** Oldest match first, one slot per match; undefined where FACEIT had no measurement. */
  values: (number | undefined)[];
  min: number | undefined;
  max: number | undefined;
};

/** The API lists matches newest first; a trend reads left to right, so it is reversed here. */
export function faceitTrend(matches: FaceitMatch[], pick: (stats: FaceitMatchStats) => number | undefined): FaceitTrend {
  const values = [...matches].reverse().map((match) => {
    const value = match.stats ? pick(match.stats) : undefined;
    return value !== undefined && Number.isFinite(value) ? value : undefined;
  });
  const known = values.filter((value): value is number => value !== undefined);
  return {
    values,
    min: known.length === 0 ? undefined : Math.min(...known),
    max: known.length === 0 ? undefined : Math.max(...known),
  };
}

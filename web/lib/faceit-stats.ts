import type { FaceitMatch } from './api/faceit.ts';

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

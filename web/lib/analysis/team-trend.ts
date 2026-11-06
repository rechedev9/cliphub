import type { LibraryMatch, LibraryMatchPlayer } from '../api/library.ts';

/** A team's recent matches as one line per player: the team page's recent performance chart. */

export const TEAM_TREND_METRICS = ['kpr', 'adr', 'kd'] as const;
export type TeamTrendMetric = (typeof TEAM_TREND_METRICS)[number];

export const TEAM_TREND_LABEL: Readonly<Record<TeamTrendMetric, string>> = {
  kpr: 'Bajas por ronda',
  adr: 'ADR',
  kd: 'K/D',
};

export type TeamTrendLine = { id: string; label: string; values: (number | null)[]; matches: number };
export type TeamTrend = { matches: LibraryMatch[]; team: (number | null)[]; players: TeamTrendLine[] };

function metricOf(metric: TeamTrendMetric, line: Pick<LibraryMatchPlayer, 'kills' | 'deaths' | 'damage' | 'rounds'>): number | null {
  if (line.rounds <= 0) return null;
  if (metric === 'kpr') return line.kills / line.rounds;
  if (metric === 'adr') return line.damage / line.rounds;
  return line.deaths > 0 ? line.kills / line.deaths : line.kills;
}

/**
 * The team's matches oldest first with its own players' value in each; a player
 * absent from a match gets a gap. Players with fewer than `minMatches` drop out.
 */
export function teamTrend(
  matches: readonly LibraryMatch[],
  teamId: string,
  options: { metric: TeamTrendMetric; minMatches?: number },
): TeamTrend {
  const ordered = [...matches].sort((a, b) => Date.parse(a.played_at) - Date.parse(b.played_at));
  const lines = new Map<string, TeamTrendLine>();
  const team: (number | null)[] = [];
  ordered.forEach((match, index) => {
    const key = match.teams.find((entry) => entry.team_id === teamId)?.key;
    const own = (match.players ?? []).filter((player) => player.team_key === key);
    const sum = own.reduce(
      (acc, p) => ({ kills: acc.kills + p.kills, deaths: acc.deaths + p.deaths, damage: acc.damage + p.damage, rounds: acc.rounds + p.rounds }),
      { kills: 0, deaths: 0, damage: 0, rounds: 0 },
    );
    team.push(key === undefined ? null : metricOf(options.metric, sum));
    for (const player of own) {
      const line = lines.get(player.steamid64) ?? {
        id: player.steamid64,
        label: player.name,
        values: Array.from({ length: ordered.length }, () => null),
        matches: 0,
      };
      line.values[index] = metricOf(options.metric, player);
      line.matches += 1;
      line.label = player.name || line.label;
      lines.set(player.steamid64, line);
    }
  });
  const players = [...lines.values()]
    .filter((line) => line.matches >= (options.minMatches ?? 1))
    .sort((a, b) => b.matches - a.matches || a.label.localeCompare(b.label, 'es'));
  return { matches: ordered, team, players };
}

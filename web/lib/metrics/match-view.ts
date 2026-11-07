import { LIBRARY_TEAM_KEYS, type LibraryMatchPlayer, type LibraryTeamKey } from '../api/library.ts';
import type { MetricsReport, PlayerMetrics, RoundTimeline, WinPoint } from '../api/metrics.ts';
import type { TacticalSide } from '../api/tactical.ts';

/**
 * The view model of one partida's Métricas tab: each round's win chance at the
 * start for one team, the kills that moved the match most and one row per player.
 */

const decimal1 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 1, maximumFractionDigits: 1 });
const decimal2 = new Intl.NumberFormat('es-ES', { minimumFractionDigits: 2, maximumFractionDigits: 2 });

export function otherTeam(key: LibraryTeamKey): LibraryTeamKey {
  return key === 'ct-start' ? 't-start' : 'ct-start';
}

/** What happened at one point of a round's win probability, in the words of the second-by-second list. */
export function winPointText(point: WinPoint, names: ReadonlyMap<number, string>): string {
  const actor = point.actor_slot === undefined ? '' : (names.get(point.actor_slot) ?? `Jugador ${point.actor_slot}`);
  const target = point.target_slot === undefined ? '' : (names.get(point.target_slot) ?? `Jugador ${point.target_slot}`);
  switch (point.kind) {
    case 'start':
      return 'Fin del tiempo de compra';
    case 'end':
      return 'Fin de la ronda';
    case 'kill':
      return actor ? `${actor} mata a ${target}` : `${target} muere`;
    case 'plant':
      return actor ? `${actor} planta la bomba` : 'Bomba plantada';
    case 'defuse':
      return actor ? `${actor} desactiva la bomba` : 'Bomba desactivada';
    case 'explode':
      return 'Explota la bomba';
    default:
      return point.kind;
  }
}

/** Halves of 12 rounds, then overtime halves of 3, the CS2 format. */
function scheduleHalf(round: number): number {
  if (round <= 24) return round <= 12 ? 1 : 2;
  return 3 + Math.floor((round - 25) / 3);
}

/** The format's guess when a half has no kill to read: overtime starts on the sides of the second half. */
function scheduledCtTeam(round: number): LibraryTeamKey {
  const half = scheduleHalf(round);
  const block = half <= 2 ? half : half - 1;
  return block % 2 === 1 ? 'ct-start' : 't-start';
}

/** The side that lost a player at this point: the alive count that dropped from the point before. */
function victimSide(point: WinPoint, before: WinPoint): TacticalSide | undefined {
  if (point.alive_ct < before.alive_ct) return 'CT';
  if (point.alive_t < before.alive_t) return 'T';
  return undefined;
}

/**
 * The team on CT in every round of the timelines. Each kill names its victim's
 * team and the side that lost a player; a round without one borrows its half's answer.
 */
export function ctTeamByRound(
  timelines: readonly RoundTimeline[],
  players: readonly Pick<LibraryMatchPlayer, 'slot' | 'team_key'>[],
): ReadonlyMap<number, LibraryTeamKey> {
  const teamOfSlot = new Map(players.map((p) => [p.slot, p.team_key]));
  const read = new Map<number, LibraryTeamKey>();
  for (const timeline of timelines) {
    let votes = 0;
    timeline.points.forEach((point, i) => {
      const before = timeline.points[i - 1];
      if (point.kind !== 'kill' || before === undefined || point.target_slot === undefined) return;
      const team = teamOfSlot.get(point.target_slot);
      const side = victimSide(point, before);
      if (team === undefined || side === undefined) return;
      votes += (team === 'ct-start') === (side === 'CT') ? 1 : -1;
    });
    if (votes !== 0) read.set(timeline.round, votes > 0 ? 'ct-start' : 't-start');
  }
  const result = new Map<number, LibraryTeamKey>();
  for (const { round } of timelines) {
    const half = [...read].find(([other]) => scheduleHalf(other) === scheduleHalf(round));
    result.set(round, read.get(round) ?? half?.[1] ?? scheduledCtTeam(round));
  }
  return result;
}

/** One kill and how much it moved the CT win chance, from -1 to 1. */
export type KillSwing = { round: number; tick: number; seconds: number; point: WinPoint; delta: number };

function killSwings(timeline: { readonly round: number; readonly points: readonly WinPoint[] }): KillSwing[] {
  return timeline.points.flatMap((point, i) => {
    const before = timeline.points[i - 1];
    if (point.kind !== 'kill' || before === undefined) return [];
    return [{ round: timeline.round, tick: point.tick, seconds: point.seconds, point, delta: point.probability_ct - before.probability_ct }];
  });
}

/** The kill of a round that moved the CT chance most, the earlier one on a tie; none when no kill moved it. */
export function strongestKill(timeline: { readonly round: number; readonly points: readonly WinPoint[] }): KillSwing | undefined {
  return killSwings(timeline).reduce<KillSwing | undefined>(
    (best, next) => (next.delta !== 0 && (best === undefined || Math.abs(next.delta) > Math.abs(best.delta)) ? next : best),
    undefined,
  );
}

/** The kills of the match that moved the win chance most, the biggest first. */
export function topSwings(timelines: readonly RoundTimeline[], limit: number): KillSwing[] {
  return timelines
    .flatMap(killSwings)
    .sort((a, b) => Math.abs(b.delta) - Math.abs(a.delta) || a.round - b.round || a.tick - b.tick)
    .slice(0, limit);
}

/** The team a swing favoured: the CT team when the CT chance went up. */
export function favouredTeam(ctTeams: ReadonlyMap<number, LibraryTeamKey>, swing: KillSwing): LibraryTeamKey {
  const ct = ctTeams.get(swing.round) ?? scheduledCtTeam(swing.round);
  return swing.delta > 0 ? ct : otherTeam(ct);
}

/** One round from one team's side: its win chance at the start and how the round ended for it. */
export type RoundBar = {
  round: number;
  side: TacticalSide;
  chance: number;
  outcome: 'win' | 'loss' | 'none';
  /** The team changed side since the round before: halftime or an overtime half. */
  halfStart: boolean;
  swing: KillSwing | undefined;
};

export function roundBars({
  timelines,
  ctTeams,
  team,
}: {
  timelines: readonly RoundTimeline[];
  ctTeams: ReadonlyMap<number, LibraryTeamKey>;
  team: LibraryTeamKey;
}): RoundBar[] {
  const rounds = timelines.filter((t) => t.points.length > 0).sort((a, b) => a.round - b.round);
  let previous: TacticalSide | undefined;
  return rounds.map((timeline) => {
    const side: TacticalSide = (ctTeams.get(timeline.round) ?? scheduledCtTeam(timeline.round)) === team ? 'CT' : 'T';
    const start = timeline.points.find((p) => p.kind === 'start') ?? timeline.points[0];
    const ct = start?.probability_ct ?? 0.5;
    let outcome: RoundBar['outcome'] = 'none';
    if (timeline.winner !== '') outcome = timeline.winner === side ? 'win' : 'loss';
    const swing = strongestKill(timeline);
    const halfStart = previous !== undefined && previous !== side;
    previous = side;
    return { round: timeline.round, side, chance: side === 'CT' ? ct : 1 - ct, outcome, halfStart, swing };
  });
}

export const GENERAL_COLUMNS = ['kast', 'adr', 'kpr', 'opening', 'trades', 'utility'] as const;
export type GeneralColumn = (typeof GENERAL_COLUMNS)[number];

/** A player's line in the General table; a null value had nothing to measure. */
export type PlayerRow = {
  player: PlayerMetrics;
  teamKey: LibraryTeamKey | undefined;
  values: Record<GeneralColumn, number | null>;
};

function perRound(value: number, rounds: number): number | null {
  return rounds > 0 ? value / rounds : null;
}

function playerValues(p: PlayerMetrics): Record<GeneralColumn, number | null> {
  const openings = p.box.opening_kills + p.box.opening_deaths;
  const utility = p.utility.he.enemy_damage + p.utility.molotov.enemy_damage;
  return {
    kast: perRound(p.box.kast_rounds, p.rounds),
    adr: perRound(p.box.damage, p.rounds),
    kpr: perRound(p.kills, p.rounds),
    opening: perRound(p.box.opening_kills, openings),
    trades: p.trades.trade_kills.count,
    utility: p.utility.measured ? perRound(utility, p.box.utility_rounds) : null,
  };
}

/** Every player of the report with their team in the match, the most impact first. */
export function playerRows(report: MetricsReport, players: readonly Pick<LibraryMatchPlayer, 'steamid64' | 'team_key'>[]): PlayerRow[] {
  const teams = new Map(players.map((p) => [p.steamid64, p.team_key]));
  return report.players
    .map((player) => {
      const raw = teams.get(player.steamid64) ?? player.match_lines[0]?.team_key;
      return { player, teamKey: LIBRARY_TEAM_KEYS.find((key) => key === raw), values: playerValues(player) };
    })
    .sort((a, b) => b.player.swing.per_round - a.player.swing.per_round);
}

/** The rows of each team, the team that started CT first, then any player without a team. */
export function teamGroups(rows: readonly PlayerRow[]): { teamKey: LibraryTeamKey | undefined; rows: PlayerRow[] }[] {
  return [...LIBRARY_TEAM_KEYS, undefined]
    .map((teamKey) => ({ teamKey, rows: rows.filter((row) => row.teamKey === teamKey) }))
    .filter((group) => group.rows.length > 0);
}

/** The best value of each column, or null when no player scored above zero in it. */
export function bestValues(rows: readonly PlayerRow[]): Record<GeneralColumn, number | null> {
  const best = (column: GeneralColumn): number | null => {
    const values = rows.map((row) => row.values[column]).filter((value): value is number => value !== null);
    const top = Math.max(0, ...values);
    return top > 0 ? top : null;
  };
  return { kast: best('kast'), adr: best('adr'), kpr: best('kpr'), opening: best('opening'), trades: best('trades'), utility: best('utility') };
}

export function isBest(value: number | null, best: number | null): boolean {
  return value !== null && best !== null && Math.abs(value - best) < 1e-9;
}

const EMPTY_TEXT: Record<GeneralColumn, string> = {
  kast: 'Sin rondas',
  adr: 'Sin rondas',
  kpr: 'Sin rondas',
  opening: 'Sin duelos',
  trades: '0',
  utility: 'Sin medir',
};

export function generalText(column: GeneralColumn, value: number | null): string {
  if (value === null) return EMPTY_TEXT[column];
  switch (column) {
    case 'kast':
      return `${decimal1.format(value * 100)} %`;
    case 'opening':
      return `${Math.round(value * 100)} %`;
    case 'kpr':
      return decimal2.format(value);
    case 'trades':
      return String(value);
    default:
      return decimal1.format(value);
  }
}

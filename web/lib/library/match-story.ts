import { LIBRARY_TEAM_KEYS, type LibraryTeamKey } from '../api/library.ts';
import { TACTICAL_BUY_TYPES, TACTICAL_ROUND_TAGS, TACTICAL_SIDES, TACTICAL_SITES } from '../api/tactical.ts';
import type { TacticalRound, TacticalSide } from '../api/tactical.ts';
import type { RoundRow } from './rounds.ts';

/**
 * The Resumen of a partida read from its rounds: the blocks of the round
 * history, each team's record by side, and the few facts that tell how it went.
 */

/** The parts of a Rondas row the Resumen reads; a RoundRow fits it as is. */
export type StoryRound = Pick<RoundRow, 'number' | 'winnerSide' | 'winnerTeamKey'> & {
  readonly round: Pick<TacticalRound, 'half' | 'overtime' | 'end_reason'> & {
    readonly economy: Pick<TacticalRound['economy'], 'ct_buy' | 't_buy'>;
    readonly bomb?: Pick<NonNullable<TacticalRound['bomb']>, 'site'>;
    readonly class: { readonly tags: readonly string[] };
  };
};

export type TeamNames = Readonly<Record<LibraryTeamKey, string>>;

/** One stretch of the round history between two breaks: a regulation half or an overtime. */
export type RoundBlock<R extends StoryRound> = {
  readonly key: string;
  readonly label: string;
  readonly rounds: readonly R[];
  /** Rounds won in this block, in LIBRARY_TEAM_KEYS order. */
  readonly wins: readonly [number, number];
};

export type SideRecord = Readonly<Record<TacticalSide, { readonly won: number; readonly played: number }>>;

/** One line of "Cómo fue la partida": the rounds it is about and what happened in them. */
export type StoryFact = { readonly rounds: string; readonly text: string };

/** A shorter run is the normal back and forth of a match, not a story. */
const MIN_STREAK = 3;

const COUNT_WORDS = ['cero', 'una', 'dos', 'tres', 'cuatro', 'cinco', 'seis', 'siete', 'ocho', 'nueve', 'diez'];
const OVERTIME_ORDINALS = ['', 'primera', 'segunda', 'tercera', 'cuarta', 'quinta', 'sexta'];

function countWord(n: number): string {
  return COUNT_WORDS[n] ?? String(n);
}

function overtimeName(overtime: number): string {
  const ordinal = OVERTIME_ORDINALS[overtime];
  return ordinal ? `la ${ordinal} prórroga` : `la prórroga ${overtime}`;
}

function isTeamKey(key: string): key is LibraryTeamKey {
  return LIBRARY_TEAM_KEYS.some((team) => team === key);
}

function rivalKey(key: LibraryTeamKey): LibraryTeamKey {
  return key === 'ct-start' ? 't-start' : 'ct-start';
}

function otherSide(side: TacticalSide): TacticalSide {
  return side === TACTICAL_SIDES.ct ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

function blockKey(round: StoryRound): string {
  const overtime = round.round.overtime ?? 0;
  return overtime > 0 ? `ot-${overtime}` : `half-${round.round.half}`;
}

function regulationLabel(half: number): string {
  if (half === 1) return 'Primera mitad';
  if (half === 2) return 'Segunda mitad';
  return `Mitad ${half}`;
}

function winsOf(rounds: readonly StoryRound[]): [number, number] {
  const wins: [number, number] = [0, 0];
  for (const round of rounds) {
    if (round.winnerTeamKey === LIBRARY_TEAM_KEYS[0]) wins[0]++;
    if (round.winnerTeamKey === LIBRARY_TEAM_KEYS[1]) wins[1]++;
  }
  return wins;
}

function ownScore(wins: readonly [number, number], team: LibraryTeamKey): readonly [number, number] {
  return team === LIBRARY_TEAM_KEYS[0] ? wins : [wins[1], wins[0]];
}

/** The round history cut at every halftime and before every overtime, in play order. */
export function roundBlocks<R extends StoryRound>(rounds: readonly R[]): RoundBlock<R>[] {
  const groups: { key: string; rounds: R[] }[] = [];
  for (const round of rounds) {
    const key = blockKey(round);
    const last = groups.at(-1);
    if (last !== undefined && last.key === key) last.rounds.push(round);
    else groups.push({ key, rounds: [round] });
  }
  const overtimes = groups.filter((group) => group.key.startsWith('ot-')).length;
  let overtime = 0;
  return groups.map((group) => {
    const first = group.rounds[0];
    let label = first === undefined ? '' : regulationLabel(first.round.half);
    if (group.key.startsWith('ot-')) {
      overtime++;
      label = overtimes > 1 ? `Prórroga ${overtime}` : 'Prórroga';
    }
    return { key: group.key, label, rounds: group.rounds, wins: winsOf(group.rounds) };
  });
}

/** Rounds a team won and played on each side; a round with no winner tells no side and is left out. */
export function sideRecord(rounds: readonly StoryRound[], team: LibraryTeamKey): SideRecord {
  const record = { CT: { won: 0, played: 0 }, T: { won: 0, played: 0 } };
  for (const round of rounds) {
    if (round.winnerSide === '' || !isTeamKey(round.winnerTeamKey)) continue;
    const won = round.winnerTeamKey === team;
    const side = won ? round.winnerSide : otherSide(round.winnerSide);
    record[side].played++;
    if (won) record[side].won++;
  }
  return record;
}

type RankedFact = StoryFact & { readonly at: number; readonly rank: number };

function pistolFact(rounds: readonly StoryRound[], names: TeamNames): RankedFact | null {
  const pistols = rounds.filter((round) => round.round.class.tags.includes(TACTICAL_ROUND_TAGS.pistol) && isTeamKey(round.winnerTeamKey));
  const first = pistols[0];
  if (first === undefined) return null;
  const numbers = pistols.map((round) => round.number);
  const label = `R ${numbers.slice(0, -1).join(', ')}${numbers.length > 1 ? ' y ' : ''}${numbers.at(-1) ?? ''}`;
  const winner = (round: StoryRound): string => (isTeamKey(round.winnerTeamKey) ? names[round.winnerTeamKey] : '');
  let text: string;
  if (pistols.every((round) => round.winnerTeamKey === first.winnerTeamKey)) {
    text = pistols.length === 1 ? `${winner(first)} gana la pistola.` : `${winner(first)} gana las ${countWord(pistols.length)} pistolas.`;
  } else {
    const rest = pistols.slice(1).map((round) => `${winner(round)} la de la ronda ${round.number}`);
    text = `${winner(first)} gana la pistola de la ronda ${first.number}${rest.length > 1 ? ', ' : ''}${rest.slice(0, -1).join(', ')} y ${rest.at(-1) ?? ''}.`;
  }
  return { rounds: label, text, at: first.number, rank: 0 };
}

/** Each team's longest run of won rounds, the first one when two are as long. */
function longestRuns(rounds: readonly StoryRound[]): Map<LibraryTeamKey, { start: number; end: number }> {
  const best = new Map<LibraryTeamKey, { start: number; end: number }>();
  let start = 0;
  for (let index = 0; index <= rounds.length; index++) {
    const team = rounds[start]?.winnerTeamKey ?? '';
    if (index < rounds.length && rounds[index]?.winnerTeamKey === team) continue;
    if (isTeamKey(team)) {
      const current = best.get(team);
      if (current === undefined || index - start > current.end - current.start + 1) best.set(team, { start, end: index - 1 });
    }
    start = index;
  }
  return best;
}

function streakFact(options: { rounds: readonly StoryRound[]; names: TeamNames; team: LibraryTeamKey; start: number; end: number }): RankedFact | null {
  const { rounds, names, team, start, end } = options;
  const run = rounds.slice(start, end + 1);
  const first = run[0];
  const last = run.at(-1);
  if (first === undefined || last === undefined || run.length < MIN_STREAK) return null;
  const name = names[team];
  const rival = names[rivalKey(team)];
  const side = run.every((round) => round.winnerSide === first.winnerSide) && first.winnerSide !== '' ? ` como ${first.winnerSide}` : '';
  const ecos = run.filter((round) => {
    if (round.winnerSide === '') return false;
    const loser = otherSide(round.winnerSide);
    return (loser === TACTICAL_SIDES.ct ? round.round.economy.ct_buy : round.round.economy.t_buy) === TACTICAL_BUY_TYPES.eco;
  }).length;
  let eco = '';
  if (ecos === run.length) eco = `, todas con ${rival} en eco`;
  else if (ecos === 1) eco = `, una de ellas con ${rival} en eco`;
  else if (ecos > 1) eco = `, ${countWord(ecos)} de ellas con ${rival} en eco`;
  const label = `R ${first.number} a ${last.number}`;
  const base = { rounds: label, at: first.number, rank: 2 };

  if (end === rounds.length - 1) {
    const text = start === 0 ? `${name} gana todas las rondas${side}${eco}.` : `${name} gana las ${countWord(run.length)} últimas rondas${side}${eco}.`;
    return { ...base, text };
  }
  const [own, other] = ownScore(winsOf(rounds.slice(0, end + 1)), team);
  let score = `recorta hasta el ${own}-${other}`;
  if (own > other) score = `se pone ${own}-${other}`;
  if (own === other) score = `empata a ${own}`;
  const firstOvertime = first.round.overtime ?? 0;
  const lastOvertime = last.round.overtime ?? 0;
  let gap = '';
  if (blockKey(first) !== blockKey(last)) gap = ' tras el descanso';
  if (lastOvertime > 0 && firstOvertime === 0) gap = ' ya en la prórroga';
  if (lastOvertime > firstOvertime && firstOvertime > 0) gap = ` ya en ${overtimeName(lastOvertime)}`;
  return { ...base, text: `${name} encadena ${countWord(run.length)} rondas${side}${eco}${eco === '' ? '' : ','} y ${score}${gap}.` };
}

function overtimeFact(rounds: readonly StoryRound[]): RankedFact | null {
  const index = rounds.findIndex((round) => (round.round.overtime ?? 0) > 0);
  const first = rounds[index];
  if (first === undefined) return null;
  const [tied] = winsOf(rounds.slice(0, index));
  const overtimes = new Set(rounds.map((round) => round.round.overtime ?? 0).filter((overtime) => overtime > 0)).size;
  const decided = overtimes > 1 ? ` y se decide en ${overtimeName(overtimes)}` : '';
  return {
    rounds: `R ${first.number}`,
    text: `Empate a ${tied} al final del tiempo reglamentario: la partida se va a la prórroga${decided}.`,
    at: first.number,
    rank: 1,
  };
}

function bombSite(round: StoryRound): string {
  const site = round.round.bomb?.site;
  return site === TACTICAL_SITES.a || site === TACTICAL_SITES.b ? ` en ${site}` : '';
}

function endPhrase(round: StoryRound): string {
  switch (round.round.end_reason) {
    case 'bomb_defused':
      return ` desactivando la bomba${bombSite(round)}`;
    case 'bomb_exploded':
      return ` haciendo explotar la bomba${bombSite(round)}`;
    case 'ct_eliminated_t':
    case 't_eliminated_ct':
      return ' eliminando al rival';
    case 'time_expired':
      return ' al agotarse el tiempo';
    case 't_surrender':
    case 'ct_surrender':
      return ' por rendición del rival';
    default:
      return '';
  }
}

function closingFact(rounds: readonly StoryRound[], names: TeamNames): RankedFact | null {
  // Only the round that really ended the match can be told as the last one.
  const last = rounds.at(-1);
  if (last === undefined || !isTeamKey(last.winnerTeamKey)) return null;
  const wins = winsOf(rounds);
  const team = last.winnerTeamKey;
  const [own, other] = ownScore(wins, team);
  const base = { rounds: `R ${last.number}`, at: last.number, rank: 3 };
  const opening = `${names[team]} gana la última ronda${endPhrase(last)}`;
  if (own > other) return { ...base, text: `${opening} y cierra ${own}-${other}.` };
  if (own === other) return { ...base, text: `${opening} y la partida acaba en empate a ${own}.` };
  return { ...base, text: `${opening}, pero ${names[rivalKey(team)]} se lleva la partida ${other}-${own}.` };
}

/** "Cómo fue la partida": the pistols, each team's longest run, the overtime and the close, in round order. */
export function matchStory(rounds: readonly StoryRound[], names: TeamNames): StoryFact[] {
  const facts: (RankedFact | null)[] = [pistolFact(rounds, names), overtimeFact(rounds), closingFact(rounds, names)];
  for (const [team, run] of longestRuns(rounds)) facts.push(streakFact({ rounds, names, team, ...run }));
  return facts
    .filter((fact) => fact !== null)
    .sort((a, b) => a.at - b.at || a.rank - b.rank)
    .map(({ rounds: label, text }) => ({ rounds: label, text }));
}

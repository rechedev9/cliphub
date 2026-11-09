import type { GrenadeRound, TeamGrenade } from '../api/grenades.ts';
import { PLAYBOOK_CATEGORIES } from '../api/playbook.ts';
import { TACTICAL_SITE_ORDER, type TacticalBuyType, type TacticalSide } from '../api/tactical.ts';
import { buyLabel, siteLabel } from '../tactical-labels.ts';
import { playbookCategoryLabel } from './playbook.ts';

/**
 * What a team does after a grenade, read from the server's per grenade facts
 * (`tacticalplan.FollowUpOf` in Go, which this mirrors for the throws the
 * finder keeps), and the round counts every figure of the finder is over.
 */

/** The window the server reads after each grenade, and how long a player must stay in a callout. */
export const FOLLOW_UP_SECONDS = 20;
export const FOLLOW_UP_STAY_SECONDS = 3;

export type FollowUpKind = 'play' | 'fight' | 'zone';
const KIND_ORDER: readonly FollowUpKind[] = ['play', 'fight', 'zone'];

/** One thing the team did after the grenade, in how many of its rounds and how many of those it won. */
export type FollowUpFact = { kind: FollowUpKind; value: string; rounds: number; won: number };
export type FollowUp = { rounds: number; won: number; facts: FollowUpFact[] };

export function roundKey(matchId: string, round: number): string {
  return `${matchId}:${round}`;
}

/** The round's category and site as `execute@A`, the category alone without a site, or empty. */
export function roundPlay(round: Pick<GrenadeRound, 'category' | 'site'>): string {
  if (round.category === '' || round.site === '' || round.site === 'none') return round.category;
  return `${round.category}@${round.site}`;
}

/** Counts what followed the grenades once per round, reading a round's first throw; every fact is over the same rounds. */
export function followUpOf(grenades: readonly TeamGrenade[], rounds: readonly GrenadeRound[]): FollowUp {
  const byRound = new Map(rounds.map((round) => [roundKey(round.match_id, round.round), round]));
  const seen = new Set<string>();
  const facts = new Map<string, FollowUpFact>();
  const out: FollowUp = { rounds: 0, won: 0, facts: [] };
  const add = (kind: FollowUpKind, value: string, won: boolean): void => {
    if (value === '') return;
    const id = `${kind}|${value}`;
    const fact = facts.get(id) ?? { kind, value, rounds: 0, won: 0 };
    fact.rounds += 1;
    if (won) fact.won += 1;
    facts.set(id, fact);
  };
  for (const grenade of grenades) {
    const key = roundKey(grenade.match_id, grenade.round);
    if (seen.has(key)) continue;
    seen.add(key);
    const round = byRound.get(key);
    const won = round?.won ?? grenade.round_won;
    out.rounds += 1;
    if (won) out.won += 1;
    if (round !== undefined) add('play', roundPlay(round), won);
    add('fight', grenade.after.fight ?? '', won);
    for (const zone of grenade.after.zones) add('zone', zone, won);
  }
  out.facts = [...facts.values()].sort(
    (a, b) => b.rounds - a.rounds || KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind) || a.value.localeCompare(b.value),
  );
  return out;
}

/** A play value in words: "Ejecución a A", or the category alone. */
export function playLabel(value: string): string {
  const [category = '', site = ''] = value.split('@');
  const known = [...PLAYBOOK_CATEGORIES.T, ...PLAYBOOK_CATEGORIES.CT].find((entry) => entry === category);
  const name = known === undefined ? category : playbookCategoryLabel(known);
  const where = TACTICAL_SITE_ORDER.find((entry) => entry === site);
  return where === undefined ? name : `${name} a ${siteLabel(where)}`;
}

/** One fact as the panel says it; `place` names a callout token. */
export function factLabel(fact: Pick<FollowUpFact, 'kind' | 'value'>, place: (token: string) => string): string {
  if (fact.kind === 'play') return playLabel(fact.value);
  if (fact.kind === 'fight') return `Primer duelo en ${place(fact.value)}`;
  return `Uno se queda en ${place(fact.value)}`;
}

/** The side every round shares, or empty when the rounds mix sides or there are none. */
export function roundsSide(rounds: readonly Pick<GrenadeRound, 'side'>[]): TacticalSide | '' {
  const sides = new Set(rounds.map((round) => round.side));
  const [only] = sides;
  return sides.size === 1 && only !== undefined ? only : '';
}

/**
 * How many of the scope's rounds a figure counts, with the scope said:
 * "8 de 31 rondas en T", or "8 de las 15 rondas en T con full" when a buy narrows them.
 */
export function denominatorText(options: { count: number; total: number; side: TacticalSide | ''; buy: TacticalBuyType | '' }): string {
  const { count, total, side, buy } = options;
  const where = side === '' ? '' : ` en ${side}`;
  const noun = total === 1 ? 'ronda' : 'rondas';
  if (buy === '') return `${count} de ${total} ${noun}${where}`;
  return `${count} de ${total === 1 ? 'la' : 'las'} ${total} ${noun}${where} con ${buyLabel(buy).toLowerCase()}`;
}

/** The rounds at least one of the grenades was thrown in. */
export function thrownRounds(grenades: readonly TeamGrenade[]): ReadonlySet<string> {
  return new Set(grenades.map((grenade) => roundKey(grenade.match_id, grenade.round)));
}

/** One round of the results: the scope's round and the grenades thrown in it, in game order. */
export type GrenadeRoundRow = { key: string; round: GrenadeRound; grenades: TeamGrenade[] };

/** The scope's rounds with the grenades, in the server's order (newest match first); with `without`, the rounds without any, as Invertir lists them. */
export function roundRows(rounds: readonly GrenadeRound[], grenades: readonly TeamGrenade[], without = false): GrenadeRoundRow[] {
  const byRound = new Map<string, TeamGrenade[]>();
  for (const grenade of grenades) {
    const key = roundKey(grenade.match_id, grenade.round);
    byRound.set(key, [...(byRound.get(key) ?? []), grenade]);
  }
  return rounds.flatMap((round) => {
    const key = roundKey(round.match_id, round.round);
    const thrown = byRound.get(key) ?? [];
    if (without ? thrown.length > 0 : thrown.length === 0) return [];
    return [{ key, round, grenades: thrown }];
  });
}

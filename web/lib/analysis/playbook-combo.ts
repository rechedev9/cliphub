import {
  PLAYBOOK_CATEGORIES,
  TACTIC_KINDS,
  type PlaybookCategoryName,
  type PlaybookCombo,
  type PlaybookGroup,
  type PlaybookRound,
  type PlaybookTactic,
  type TacticKind,
  type TeamPlaybook,
} from '../api/playbook.ts';
import type { PlayInput } from '../api/stratbook.ts';
import {
  MIN_RELIABLE_SAMPLE,
  TACTICAL_BUY_TYPE_ORDER,
  TACTICAL_SITE_ORDER,
  type TacticalBuyType,
  type TacticalRate,
  type TacticalSide,
  type TacticalSite,
} from '../api/tactical.ts';
import { MAX_PLAY_EXAMPLES } from '../stratbook/plays.ts';
import { buyLabel, siteLabel } from '../tactical-labels.ts';
import { playbookCategoryLabel } from './playbook.ts';

const ALL_CATEGORIES: readonly PlaybookCategoryName[] = [...PLAYBOOK_CATEGORIES.T, ...PLAYBOOK_CATEGORIES.CT];

function isCategory(value: string): value is PlaybookCategoryName {
  return ALL_CATEGORIES.some((category) => category === value);
}

function isSite(value: string): value is TacticalSite {
  return TACTICAL_SITE_ORDER.some((site) => site === value);
}

function isBuy(value: string): value is TacticalBuyType {
  return TACTICAL_BUY_TYPE_ORDER.some((buy) => buy === value);
}

export const EMPTY_COMBO: PlaybookCombo = { with: [], without: [] };

export type ComboMode = 'with' | 'without';

/** Where a key sits in the combination, or null when it is not in it. */
export function comboMode(combo: PlaybookCombo, key: string): ComboMode | null {
  if (combo.with.includes(key)) return 'with';
  if (combo.without.includes(key)) return 'without';
  return null;
}

/** Puts a key in `mode`, or takes it out when it is already there; a key is never in both lists. */
export function toggleTactic(combo: PlaybookCombo, key: string, mode: ComboMode): PlaybookCombo {
  const current = comboMode(combo, key);
  const next = { with: combo.with.filter((entry) => entry !== key), without: combo.without.filter((entry) => entry !== key) };
  if (current === mode) return next;
  return { ...next, [mode]: [...next[mode], key] };
}

/** The combination in a page's query (`with` and `without`, repeated), keeping only keys the playbook knows. */
export function comboFromQuery(query: string, known: ReadonlySet<string>): PlaybookCombo {
  const params = new URLSearchParams(query);
  const keep = (keys: string[]): string[] => [...new Set(keys.filter((key) => known.has(key)))];
  const withKeys = keep(params.getAll('with'));
  return { with: withKeys, without: keep(params.getAll('without')).filter((key) => !withKeys.includes(key)) };
}

/** The page's query with the combination written over its `with` and `without`. */
export function writeComboQuery(combo: PlaybookCombo, query: string): string {
  const params = new URLSearchParams(query);
  params.delete('with');
  params.delete('without');
  for (const key of combo.with) params.append('with', key);
  for (const key of combo.without) params.append('without', key);
  return params.toString();
}

export function comboIsEmpty(combo: PlaybookCombo): boolean {
  return combo.with.length === 0 && combo.without.length === 0;
}

export function comboKeeps(tactics: readonly string[], combo: PlaybookCombo): boolean {
  return combo.with.every((key) => tactics.includes(key)) && !combo.without.some((key) => tactics.includes(key));
}

function rate(count: number, total: number): TacticalRate {
  return { count, total, pct: total > 0 ? Math.round((count / total) * 1000) / 10 : 0, reliable: total >= MIN_RELIABLE_SAMPLE };
}

function group(rounds: readonly PlaybookRound[], total: number): PlaybookGroup {
  const wins = rounds.filter((round) => round.won).length;
  return {
    rounds: rounds.length,
    share: rate(rounds.length, total),
    win_rate: rate(wins, rounds.length),
    refs: rounds.map((round) => ({ match_id: round.match_id, round: round.round, tick: round.tick })),
  };
}

function median(values: readonly number[]): number {
  const sorted = [...values].sort((a, b) => a - b);
  if (sorted.length === 0) return 0;
  const mid = Math.floor(sorted.length / 2);
  return sorted.length % 2 === 1 ? (sorted[mid] ?? 0) : ((sorted[mid - 1] ?? 0) + (sorted[mid] ?? 0)) / 2;
}

function bucketBy<K>(rounds: readonly PlaybookRound[], keysOf: (round: PlaybookRound) => readonly K[]): Map<K, PlaybookRound[]> {
  const out = new Map<K, PlaybookRound[]>();
  for (const round of rounds) {
    for (const key of keysOf(round)) out.set(key, [...(out.get(key) ?? []), round]);
  }
  return out;
}

function tacticOrder(a: PlaybookTactic, b: PlaybookTactic): number {
  const byKind = TACTIC_KINDS.indexOf(a.kind) - TACTIC_KINDS.indexOf(b.kind);
  if (byKind !== 0) return byKind;
  if (a.rounds !== b.rounds) return b.rounds - a.rounds;
  return a.key < b.key ? -1 : 1;
}

/**
 * The playbook counted again over the rounds the combination keeps, as the
 * server does for `with` and `without`. Tactic names come from the full
 * playbook, so a tactic keeps its label while a combination hides it.
 */
export function narrowPlaybook(playbook: TeamPlaybook, combo: PlaybookCombo): TeamPlaybook {
  if (comboIsEmpty(combo)) return playbook;
  const rounds = playbook.rounds.filter((round) => comboKeeps(round.tactics, combo));
  const total = rounds.length;
  const framed = rounds.filter((round) => round.framed).length;
  const byCategory = bucketBy(rounds, (round) => [round.category]);
  const bySite = bucketBy(rounds, (round) => [round.site]);
  const byZone = bucketBy(rounds, (round) => [...new Set(round.zones.map((hold) => hold.zone))]);
  const byTactic = bucketBy(rounds, (round) => round.tactics);
  const known = new Map(playbook.tactics.map((tactic) => [tactic.key, tactic]));
  const zoneLabels = new Map(playbook.zones.map((zone) => [zone.zone, zone.label]));
  const categories: readonly PlaybookCategoryName[] = PLAYBOOK_CATEGORIES[playbook.side];
  const tempos = rounds.flatMap((round) => (round.tempo_seconds === null ? [] : [round.tempo_seconds]));
  return {
    ...playbook,
    combo,
    round_count: total,
    win_rate: rate(rounds.filter((round) => round.won).length, total),
    framed,
    rounds,
    tempo: { ...playbook.tempo, samples: tempos.length, median: median(tempos), buckets: [] },
    categories: categories.map((category) => ({ category, ...group(byCategory.get(category) ?? [], total) })),
    sites: playbook.sites.map((site) => ({ site: site.site, ...group(bySite.get(site.site) ?? [], total) })),
    zones: [...byZone.entries()]
      .map(([zone, held]) => ({ zone, label: zoneLabels.get(zone) ?? zone, ...group(held, framed) }))
      .sort((a, b) => b.rounds - a.rounds || (a.zone < b.zone ? -1 : 1)),
    tactics: [...byTactic.entries()]
      .flatMap(([key, shown]) => {
        const base = known.get(key);
        return base === undefined ? [] : [{ ...base, ...group(shown, total) }];
      })
      .sort(tacticOrder),
  };
}

const KIND_TITLES: Record<TacticKind, string> = {
  category: 'Tipo de ronda',
  site: 'Dónde acaba',
  buy: 'Su compra',
  opponent_buy: 'Compra rival',
  opening: 'Primer duelo',
  opening_result: 'Resultado del primer duelo',
  awp: 'AWP al empezar',
  zone: 'Zonas que controlan',
  utility: 'Humos y molotovs',
};

export function tacticKindTitle(kind: TacticKind): string {
  return KIND_TITLES[kind];
}

/** What a kind's rows mean, for the line under each card title. */
export function tacticKindHint(kind: TacticKind, awpSeconds: number): string {
  switch (kind) {
    case 'opening':
      return 'Dónde estaba su jugador en el primer duelo de la ronda.';
    case 'awp':
      return `Dónde estaba su AWP a los ${awpSeconds} s.`;
    case 'utility':
      return 'Dónde cayeron antes de plantar o de acabar la ronda.';
    case 'zone':
      return 'Zonas que sujetaron al principio de la ronda.';
    default:
      return '';
  }
}

/** Seconds after freeze end the AWP spot is read at (`tacticalplan.AWPStartSeconds`). */
export const AWP_START_SECONDS = 12;

function place(tactic: Pick<PlaybookTactic, 'label' | 'value'>): string {
  return tactic.label !== undefined && tactic.label !== '' ? tactic.label : tactic.value;
}

/** "Humo en Ventana", one tactic as a short phrase. */
export function tacticLabel(tactic: Pick<PlaybookTactic, 'kind' | 'value' | 'label' | 'grenade'>): string {
  switch (tactic.kind) {
    case 'category':
      return isCategory(tactic.value) ? playbookCategoryLabel(tactic.value) : tactic.value;
    case 'site':
      if (!isSite(tactic.value)) return tactic.value;
      return tactic.value === 'none' ? 'Ningún sitio' : `Ronda en ${siteLabel(tactic.value)}`;
    case 'buy':
      return isBuy(tactic.value) ? `Compra ${buyLabel(tactic.value).toLowerCase()}` : tactic.value;
    case 'opponent_buy':
      return isBuy(tactic.value) ? `Rival en ${buyLabel(tactic.value).toLowerCase()}` : tactic.value;
    case 'opening':
      return `Primer duelo en ${place(tactic)}`;
    case 'opening_result':
      return tactic.value === 'won' ? 'Primer duelo ganado' : 'Primer duelo perdido';
    case 'awp':
      return `AWP en ${place(tactic)}`;
    case 'zone':
      return `Control de ${place(tactic)}`;
    case 'utility':
      return `${tactic.grenade === 'molotov' ? 'Molotov' : 'Humo'} en ${place(tactic)}`;
    default:
      return tactic.value;
  }
}

/** A label after "sin", lower-cased unless it starts with an acronym such as AWP. */
function afterSin(label: string): string {
  const second = label.charAt(1);
  return second !== '' && second === second.toLowerCase() ? `${label.charAt(0).toLowerCase()}${label.slice(1)}` : label;
}

/** "Sin humo en Ventana", an excluded tactic. */
export function withoutLabel(label: string): string {
  return `Sin ${afterSin(label)}`;
}

/** "Rush + Humo en Ventana, sin AWP en Mid"; empty for no combination. */
export function comboSentence(combo: PlaybookCombo, labelOf: (key: string) => string): string {
  const parts: string[] = [];
  if (combo.with.length > 0) parts.push(combo.with.map(labelOf).join(' + '));
  if (combo.without.length > 0) parts.push(`sin ${combo.without.map((key) => afterSin(labelOf(key))).join(' ni ')}`);
  return parts.join(', ');
}

function mostCommon<T>(values: readonly T[], fallback: T): T {
  const counts = new Map<T, number>();
  let best = fallback;
  let bestCount = 0;
  for (const value of values) {
    const count = (counts.get(value) ?? 0) + 1;
    counts.set(value, count);
    if (count > bestCount) {
      best = value;
      bestCount = count;
    }
  }
  return best;
}

/**
 * A stratbook play from a combination: the rounds' most common pattern and
 * site, the combination as notes and its first rounds as examples.
 */
export function playFromCombo(
  playbook: Pick<TeamPlaybook, 'map' | 'side' | 'rounds'>,
  input: { teamId: string; title: string; sentence: string },
): PlayInput {
  const rounds = playbook.rounds;
  const side: TacticalSide = playbook.side;
  const won = rounds.filter((round) => round.won).length;
  const pct = rounds.length > 0 ? Math.round((won / rounds.length) * 100) : 0;
  return {
    title: input.title.trim(),
    team_id: input.teamId,
    map: playbook.map,
    side,
    kind: mostCommon(rounds.flatMap((round) => (round.pattern === '' ? [] : [round.pattern])), side === 'T' ? 'default' : 'hold'),
    site: mostCommon<TacticalSite>(rounds.map((round) => round.site), 'none'),
    origin: 'playbook',
    positions_seconds: 0,
    grenades: [],
    positions: [],
    notes: `Combinación del playbook: ${input.sentence}. Gana el ${pct} % de ${rounds.length} rondas.`,
    examples: rounds.slice(0, MAX_PLAY_EXAMPLES).map((round) => ({ match_id: round.match_id, round: round.round, tick: round.tick })),
  };
}

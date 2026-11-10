import type { TacticalFilter } from '../api/tactical.ts';
import { withoutOrphanOutcome } from '../tactical-filter.ts';
import { buyLabel, ctPatternLabel, outcomeLabel, phaseLabel, roundTagLabel, siteLabel, tPatternLabel } from '../tactical-labels.ts';

/** The four ways the Replay reads the rounds in its scope. */
export const REPLAY_FORMS = ['repeticion', 'calor', 'recorridos', 'granadas'] as const;
/** Forms only a team's Replay has: they read rounds across the team's matches. */
export const TEAM_ONLY_FORMS = ['patrones'] as const;
export type ReplayForm = (typeof REPLAY_FORMS)[number] | (typeof TEAM_ONLY_FORMS)[number];

export const REPLAY_FORM_LABEL: Record<ReplayForm, string> = {
  repeticion: 'Repetición',
  calor: 'Mapa de calor',
  recorridos: 'Recorridos',
  granadas: 'Granadas',
  patrones: 'Patrones',
};

/** Query key of the open form; Repetición is the default and is left out of the URL. */
export const REPLAY_FORM_QUERY = 'forma';
export const DEFAULT_REPLAY_FORM: ReplayForm = 'repeticion';

export function replayFormFromQuery(query: Pick<URLSearchParams, 'get'>): ReplayForm {
  const raw = query.get(REPLAY_FORM_QUERY);
  return REPLAY_FORMS.find((form) => form === raw) ?? DEFAULT_REPLAY_FORM;
}

/** The query with the form written over it, every other key kept. */
export function writeReplayForm(current: string, form: ReplayForm): string {
  const params = new URLSearchParams(current);
  if (form === DEFAULT_REPLAY_FORM) params.delete(REPLAY_FORM_QUERY);
  else params.set(REPLAY_FORM_QUERY, form);
  return params.toString();
}

/** One active filter as the scope bar shows it: its words and the filter without it. */
export type FilterChip = { key: string; label: string; without: TacticalFilter };

/** What the chips need to name a team and a player. */
export type ChipNames = {
  teams: readonly { key: string; name: string }[];
  players: readonly { slot: number; name: string }[];
};

function withoutValue<T>(values: readonly T[] | undefined, value: T): T[] | undefined {
  const rest = (values ?? []).filter((entry) => entry !== value);
  return rest.length === 0 ? undefined : rest;
}

/**
 * Every constraint of the filter as its own chip, in the order the bar reads:
 * who, then what kind of round, then the rest. Removing a team or a side also
 * drops a result that would be left with no one to win or lose.
 */
export function filterChips(filter: TacticalFilter, names: ChipNames): FilterChip[] {
  const chips: FilterChip[] = [];
  const add = (key: string, label: string, without: TacticalFilter): void => {
    chips.push({ key, label, without: withoutOrphanOutcome(without) });
  };
  if (filter.team_key !== undefined) {
    const team = names.teams.find((entry) => entry.key === filter.team_key);
    add('team', team === undefined || team.name === '' ? filter.team_key : team.name, { ...filter, team_key: undefined });
  }
  if (filter.side !== undefined) add('side', `Lado ${filter.side}`, { ...filter, side: undefined });
  if (filter.outcome !== undefined) add('outcome', outcomeLabel(filter.outcome), { ...filter, outcome: undefined });
  if (filter.round_from !== undefined || filter.round_to !== undefined) {
    const { round_from: from, round_to: to } = filter;
    let label = `Hasta la ronda ${to}`;
    if (from !== undefined && to !== undefined) label = `Rondas ${from} a ${to}`;
    else if (from !== undefined) label = `Desde la ronda ${from}`;
    add('rounds', label, { ...filter, round_from: undefined, round_to: undefined });
  }
  for (const pattern of filter.t_patterns ?? []) {
    add(`t:${pattern}`, `Ataque: ${tPatternLabel(pattern)}`, { ...filter, t_patterns: withoutValue(filter.t_patterns, pattern) });
  }
  for (const pattern of filter.ct_patterns ?? []) {
    add(`ct:${pattern}`, `Defensa: ${ctPatternLabel(pattern)}`, { ...filter, ct_patterns: withoutValue(filter.ct_patterns, pattern) });
  }
  if (filter.phase !== undefined) add('phase', phaseLabel(filter.phase), { ...filter, phase: undefined });
  for (const buy of filter.buys ?? []) add(`buy:${buy}`, `Compra: ${buyLabel(buy)}`, { ...filter, buys: withoutValue(filter.buys, buy) });
  for (const buy of filter.opponent_buys ?? []) {
    add(`rival:${buy}`, `Compra rival: ${buyLabel(buy)}`, { ...filter, opponent_buys: withoutValue(filter.opponent_buys, buy) });
  }
  for (const site of filter.sites ?? []) add(`site:${site}`, `Sitio: ${siteLabel(site)}`, { ...filter, sites: withoutValue(filter.sites, site) });
  for (const tag of filter.tags ?? []) add(`tag:${tag}`, roundTagLabel(tag), { ...filter, tags: withoutValue(filter.tags, tag) });
  for (const slot of filter.slots ?? []) {
    const player = names.players.find((entry) => entry.slot === slot);
    add(`slot:${slot}`, player?.name ?? `Jugador ${slot}`, { ...filter, slots: withoutValue(filter.slots, slot) });
  }
  return chips;
}

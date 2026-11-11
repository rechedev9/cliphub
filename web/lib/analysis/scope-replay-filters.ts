/**
 * The team Replay's round filters beyond buys, AWP and advantage: each one is a
 * `tacticalplan.Filter` criterion, under the query name the server reads.
 */

export const SCOPE_SITES = ['A', 'B', 'mid', 'none'] as const;
export type ScopeSite = (typeof SCOPE_SITES)[number] | '';
export type ScopeOutcome = '' | 'win' | 'loss';
export type ScopePhase = '' | 'regulation' | 'overtime';
export const SCOPE_END_REASONS = ['elimination', 'bomb_exploded', 'bomb_defused', 'time_expired'] as const;
export type ScopeEndReason = (typeof SCOPE_END_REASONS)[number] | '';
export const SCOPE_WEAPONS = ['scout', 'no_scout', 'hero_ak', 'hero_awp'] as const;
export type ScopeWeapon = (typeof SCOPE_WEAPONS)[number] | '';
/** Equipment value per player in dollars, both ends included; null leaves that end open. */
export type ScopeEquip = { readonly from: number | null; readonly to: number | null };

export type ReplayFilters = {
  readonly site: ScopeSite;
  readonly outcome: ScopeOutcome;
  readonly postPlant: boolean;
  readonly phase: ScopePhase;
  readonly secondRound: boolean;
  readonly endReason: ScopeEndReason;
  readonly weapon: ScopeWeapon;
  readonly opponentWeapon: ScopeWeapon;
  readonly equip: ScopeEquip;
  readonly opponentEquip: ScopeEquip;
  /** A Libreta play id; its example rounds are the only ones kept. */
  readonly tactic: string;
  /** Libreta play ids whose example rounds are dropped. */
  readonly excludeTactics: readonly string[];
  /** Demo tags: a match carrying one of `matchTags` is kept, one carrying any of `excludeMatchTags` is dropped. */
  readonly matchTags: readonly string[];
  readonly excludeMatchTags: readonly string[];
  /** Events of the match's series. */
  readonly events: readonly string[];
};

export const REPLAY_FILTER_PARAMS = {
  site: 'site',
  outcome: 'outcome',
  tag: 'tag',
  phase: 'phase',
  secondRound: 'second_round',
  endReason: 'end_reason',
  weapon: 'weapon',
  opponentWeapon: 'opponent_weapon',
  equipFrom: 'equip_from',
  equipTo: 'equip_to',
  opponentEquipFrom: 'opponent_equip_from',
  opponentEquipTo: 'opponent_equip_to',
  tactic: 'tactic',
  excludeTactic: 'exclude_tactic',
  matchTag: 'match_tag',
  excludeMatchTag: 'exclude_match_tag',
  event: 'event',
} as const;

/** The server's bounds on a tag or event list (`maxScopeTexts`, `maxScopeTextRunes`). */
export const SCOPE_TEXTS_MAX = 20;
const SCOPE_TEXT_RUNES_MAX = 80;

/** The `tag` value post-plant stands for (`tacticalplan.TagPostPlant`). */
export const POST_PLANT_TAG = 'postplant';
/** The server's bound on an equipment range (`tacticalplan.MaxEquipPerPlayer`). */
export const EQUIP_MAX = 20000;
const OPEN_EQUIP: ScopeEquip = { from: null, to: null };

export const DEFAULT_REPLAY_FILTERS: ReplayFilters = {
  site: '',
  outcome: '',
  postPlant: false,
  phase: '',
  secondRound: false,
  endReason: '',
  weapon: '',
  opponentWeapon: '',
  equip: OPEN_EQUIP,
  opponentEquip: OPEN_EQUIP,
  tactic: '',
  excludeTactics: [],
  matchTags: [],
  excludeMatchTags: [],
  events: [],
};

const UUID_RE = /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i;

type Query = Pick<URLSearchParams, 'get' | 'getAll'>;

function oneOf<T extends string>(list: readonly T[], value: string | null): T | '' {
  return list.find((entry) => entry === value) ?? '';
}

/** One end of an equipment range: a whole dollar amount from 1 to EQUIP_MAX, or open. */
export function equipEnd(text: string | null): number | null {
  if (text === null || !/^\d{1,5}$/.test(text)) return null;
  const value = Number(text);
  return value >= 1 && value <= EQUIP_MAX ? value : null;
}

/** A range in order, as the server takes it: reversed ends are read as the person meant them. */
export function orderedEquip(range: ScopeEquip): ScopeEquip {
  const { from, to } = range;
  return from !== null && to !== null && from > to ? { from: to, to: from } : range;
}

function parseEquip(from: string | null, to: string | null): ScopeEquip {
  return { from: equipEnd(from), to: equipEnd(to) };
}

function parseSite(value: string | null): ScopeSite {
  return SCOPE_SITES.find((site) => site.toLowerCase() === value?.toLowerCase()) ?? '';
}

export function parseReplayFilters(query: Query): ReplayFilters {
  const p = REPLAY_FILTER_PARAMS;
  const outcome = query.get(p.outcome);
  const phase = query.get(p.phase);
  const tactic = query.get(p.tactic) ?? '';
  return {
    site: parseSite(query.get(p.site)),
    outcome: outcome === 'win' || outcome === 'loss' ? outcome : '',
    postPlant: query.getAll(p.tag).includes(POST_PLANT_TAG),
    phase: phase === 'regulation' || phase === 'overtime' ? phase : '',
    secondRound: query.get(p.secondRound) === 'true' || query.get(p.secondRound) === '1',
    endReason: oneOf(SCOPE_END_REASONS, query.get(p.endReason)),
    weapon: oneOf(SCOPE_WEAPONS, query.get(p.weapon)),
    opponentWeapon: oneOf(SCOPE_WEAPONS, query.get(p.opponentWeapon)),
    equip: parseEquip(query.get(p.equipFrom), query.get(p.equipTo)),
    opponentEquip: parseEquip(query.get(p.opponentEquipFrom), query.get(p.opponentEquipTo)),
    tactic: UUID_RE.test(tactic) ? tactic.toLowerCase() : '',
    excludeTactics: textList(query.getAll(p.excludeTactic).filter((id) => UUID_RE.test(id)).map((id) => id.toLowerCase())),
    matchTags: textList(query.getAll(p.matchTag)),
    excludeMatchTags: textList(query.getAll(p.excludeMatchTag)),
    events: textList(query.getAll(p.event)),
  };
}

/** A repeated text parameter, trimmed, without blanks or repeats, inside the server's bounds. */
function textList(values: readonly string[]): string[] {
  const out: string[] = [];
  for (const raw of values) {
    const value = raw.trim();
    if (value !== '' && [...value].length <= SCOPE_TEXT_RUNES_MAX && !out.includes(value)) out.push(value);
  }
  return out.slice(0, SCOPE_TEXTS_MAX);
}

function countText(value: number | null): string {
  return value === null ? '' : String(value);
}

/** The filters as query entries, defaults left out, in the order the server reads them. */
export function replayFilterEntries(filters: ReplayFilters): [string, string][] {
  const p = REPLAY_FILTER_PARAMS;
  const entries: [string, string][] = [
    [p.site, filters.site],
    [p.outcome, filters.outcome],
    [p.tag, filters.postPlant ? POST_PLANT_TAG : ''],
    [p.phase, filters.phase],
    [p.secondRound, filters.secondRound ? 'true' : ''],
    [p.endReason, filters.endReason],
    [p.weapon, filters.weapon],
    [p.opponentWeapon, filters.opponentWeapon],
    [p.equipFrom, countText(filters.equip.from)],
    [p.equipTo, countText(filters.equip.to)],
    [p.opponentEquipFrom, countText(filters.opponentEquip.from)],
    [p.opponentEquipTo, countText(filters.opponentEquip.to)],
    [p.tactic, filters.tactic],
    ...filters.excludeTactics.map((id): [string, string] => [p.excludeTactic, id]),
    ...filters.matchTags.map((tag): [string, string] => [p.matchTag, tag]),
    ...filters.excludeMatchTags.map((tag): [string, string] => [p.excludeMatchTag, tag]),
    ...filters.events.map((event): [string, string] => [p.event, event]),
  ];
  return entries.filter(([, value]) => value !== '');
}

export function equipActive(range: ScopeEquip): boolean {
  return range.from !== null || range.to !== null;
}

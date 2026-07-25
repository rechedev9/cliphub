import {
  TACTICAL_BUY_TYPES,
  TACTICAL_CT_PATTERNS,
  TACTICAL_EVENT_KINDS,
  TACTICAL_OUTCOMES,
  TACTICAL_PHASES,
  TACTICAL_ROUND_TAGS,
  TACTICAL_SIDES,
  TACTICAL_SITES,
  TACTICAL_STATES,
  TACTICAL_T_PATTERNS,
} from './api/tactical.ts';
import type {
  TacticalBuyType,
  TacticalCTPattern,
  TacticalEventKind,
  TacticalOutcome,
  TacticalPhase,
  TacticalRoundTag,
  TacticalSide,
  TacticalSite,
  TacticalState,
  TacticalTPattern,
} from './api/tactical.ts';

/**
 * Spanish surface wording for the closed tactical vocabularies. The vocabularies
 * themselves are the wire contract and never change shape here; this module is
 * the single place a value becomes a word an analyst reads, so no component ever
 * inlines "Eco" or "Retoma" next to a raw `eco` / `retake`.
 *
 * A round's tag list is open on the wire (`Class.tags` is `[]string`), so
 * `roundTagLabel` falls back to the raw value instead of hiding a tag the
 * classifier started emitting before this table knew about it.
 */

const BUY_LABELS: Record<TacticalBuyType, string> = {
  [TACTICAL_BUY_TYPES.pistol]: 'Pistola',
  [TACTICAL_BUY_TYPES.eco]: 'Eco',
  [TACTICAL_BUY_TYPES.semi]: 'Semi',
  [TACTICAL_BUY_TYPES.force]: 'Force',
  [TACTICAL_BUY_TYPES.full]: 'Full',
  [TACTICAL_BUY_TYPES.unknown]: 'Sin dato',
};

/** The economy a side took into the round. */
export function buyLabel(buy: TacticalBuyType): string {
  return BUY_LABELS[buy];
}

const T_PATTERN_LABELS: Record<TacticalTPattern, string> = {
  [TACTICAL_T_PATTERNS.execute]: 'Ejecución',
  [TACTICAL_T_PATTERNS.default]: 'Default',
  [TACTICAL_T_PATTERNS.split]: 'Split',
  [TACTICAL_T_PATTERNS.fast]: 'Rápida',
  [TACTICAL_T_PATTERNS.ecoRush]: 'Rush de eco',
  [TACTICAL_T_PATTERNS.save]: 'Save',
  [TACTICAL_T_PATTERNS.unknown]: 'Sin forma',
};

/** The attacking side's round shape. */
export function tPatternLabel(pattern: TacticalTPattern): string {
  return T_PATTERN_LABELS[pattern];
}

const CT_PATTERN_LABELS: Record<TacticalCTPattern, string> = {
  [TACTICAL_CT_PATTERNS.hold]: 'Defensa',
  [TACTICAL_CT_PATTERNS.retake]: 'Retoma',
  [TACTICAL_CT_PATTERNS.aggression]: 'Agresión',
  [TACTICAL_CT_PATTERNS.stack]: 'Stack',
  [TACTICAL_CT_PATTERNS.save]: 'Save',
  [TACTICAL_CT_PATTERNS.unknown]: 'Sin forma',
};

/** The defending side's round shape. */
export function ctPatternLabel(pattern: TacticalCTPattern): string {
  return CT_PATTERN_LABELS[pattern];
}

/**
 * A pattern bucket's label. The aggregate types `PatternBucket.pattern` as a
 * plain string because one bucket list carries T patterns and the other CT
 * patterns, so this resolves against both vocabularies (the two values they
 * share, `save` and `unknown`, mean the same thing on either side) and falls
 * back to the raw value rather than dropping a pattern it has not learnt.
 */
export function patternLabel(pattern: string): string {
  return T_PATTERN_LOOKUP.get(pattern) ?? CT_PATTERN_LOOKUP.get(pattern) ?? pattern;
}

const T_PATTERN_LOOKUP: ReadonlyMap<string, string> = new Map(Object.entries(T_PATTERN_LABELS));
const CT_PATTERN_LOOKUP: ReadonlyMap<string, string> = new Map(Object.entries(CT_PATTERN_LABELS));

const SITE_LABELS: Record<TacticalSite, string> = {
  [TACTICAL_SITES.a]: 'A',
  [TACTICAL_SITES.b]: 'B',
  [TACTICAL_SITES.mid]: 'Medio',
  [TACTICAL_SITES.none]: 'Sin sitio',
};

/** Where the round was decided. */
export function siteLabel(site: TacticalSite): string {
  return SITE_LABELS[site];
}

const ROUND_TAG_LABELS: Record<TacticalRoundTag, string> = {
  [TACTICAL_ROUND_TAGS.postPlant]: 'Post-plant',
  [TACTICAL_ROUND_TAGS.retakeWon]: 'Retoma ganada',
  [TACTICAL_ROUND_TAGS.fullSave]: 'Save completo',
  [TACTICAL_ROUND_TAGS.ace]: 'Ace',
  [TACTICAL_ROUND_TAGS.openingTraded]: 'Entrada tradeada',
  [TACTICAL_ROUND_TAGS.antiEco]: 'Anti-eco',
  [TACTICAL_ROUND_TAGS.overtime]: 'Prórroga',
  [TACTICAL_ROUND_TAGS.pistol]: 'Pistola',
  [TACTICAL_ROUND_TAGS.timeExpired]: 'Tiempo agotado',
};

const ROUND_TAG_LOOKUP: ReadonlyMap<string, string> = new Map(Object.entries(ROUND_TAG_LABELS));

/** A round tag, falling back to the raw value for a tag this table has not learnt. */
export function roundTagLabel(tag: string): string {
  return ROUND_TAG_LOOKUP.get(tag) ?? tag;
}

const EVENT_LABELS: Record<TacticalEventKind, string> = {
  [TACTICAL_EVENT_KINDS.kill]: 'Baja',
  [TACTICAL_EVENT_KINDS.plant]: 'Plantada',
  [TACTICAL_EVENT_KINDS.defuse]: 'Desactivación',
  [TACTICAL_EVENT_KINDS.explode]: 'Explosión',
  [TACTICAL_EVENT_KINDS.smoke]: 'Humo',
  [TACTICAL_EVENT_KINDS.flash]: 'Flash',
  [TACTICAL_EVENT_KINDS.he]: 'HE',
  [TACTICAL_EVENT_KINDS.molotov]: 'Molotov',
  [TACTICAL_EVENT_KINDS.decoy]: 'Señuelo',
  [TACTICAL_EVENT_KINDS.bombDrop]: 'Bomba soltada',
};

/** One thing that happened at a tick. */
export function eventKindLabel(kind: TacticalEventKind): string {
  return EVENT_LABELS[kind];
}

const OUTCOME_LABELS: Record<TacticalOutcome, string> = {
  [TACTICAL_OUTCOMES.win]: 'Ganadas',
  [TACTICAL_OUTCOMES.loss]: 'Perdidas',
};

/** Round result, from the filter's perspective. */
export function outcomeLabel(outcome: TacticalOutcome): string {
  return OUTCOME_LABELS[outcome];
}

const PHASE_LABELS: Record<TacticalPhase, string> = {
  [TACTICAL_PHASES.regulation]: 'Reglamentario',
  [TACTICAL_PHASES.overtime]: 'Prórroga',
};

/** Regulation or overtime. */
export function phaseLabel(phase: TacticalPhase): string {
  return PHASE_LABELS[phase];
}

const STATE_LABELS: Record<TacticalState, string> = {
  [TACTICAL_STATES.none]: 'Sin analizar',
  [TACTICAL_STATES.queued]: 'En cola',
  [TACTICAL_STATES.running]: 'Analizando',
  [TACTICAL_STATES.ready]: 'Listo',
  [TACTICAL_STATES.failed]: 'Fallido',
};

/** Lifecycle of the analysis of one demo. */
export function stateLabel(state: TacticalState): string {
  return STATE_LABELS[state];
}

/** The opposing side, mirroring `tacticalplan.Side.Opponent`. */
export function opponentSide(side: TacticalSide): TacticalSide {
  return side === TACTICAL_SIDES.ct ? TACTICAL_SIDES.t : TACTICAL_SIDES.ct;
}

/**
 * Seconds as a signed round clock: negative through freeze time, so a marker at
 * `-6.2` reads as six seconds before the round went live.
 */
export function roundClockLabel(seconds: number): string {
  if (!Number.isFinite(seconds)) return '—';
  const sign = seconds < 0 ? '-' : '';
  const total = Math.abs(seconds);
  const minutes = Math.floor(total / 60);
  const rest = total - minutes * 60;
  return `${sign}${minutes}:${rest.toFixed(1).padStart(4, '0')}`;
}

/**
 * A rate as a percentage with its denominator, never one without the other:
 * `43.5 % (n=23)`. An empty sample reads as a dash rather than `0 %`.
 */
export function rateLabel(pct: number, total: number): string {
  if (total <= 0) return '— (n=0)';
  return `${pct.toFixed(1)} % (n=${total})`;
}

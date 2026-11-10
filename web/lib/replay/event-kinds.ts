import { TACTICAL_EVENT_KINDS } from '../api/tactical.ts';
import type { TacticalEventKind } from '../api/tactical.ts';

const UTILITY_KINDS: ReadonlySet<TacticalEventKind> = new Set([
  TACTICAL_EVENT_KINDS.smoke,
  TACTICAL_EVENT_KINDS.flash,
  TACTICAL_EVENT_KINDS.he,
  TACTICAL_EVENT_KINDS.molotov,
  TACTICAL_EVENT_KINDS.decoy,
]);

const BOMB_KINDS: ReadonlySet<TacticalEventKind> = new Set([
  TACTICAL_EVENT_KINDS.plant,
  TACTICAL_EVENT_KINDS.defuse,
  TACTICAL_EVENT_KINDS.explode,
  TACTICAL_EVENT_KINDS.bombDrop,
]);

export function isUtilityKind(kind: TacticalEventKind): boolean {
  return UTILITY_KINDS.has(kind);
}

export function isBombKind(kind: TacticalEventKind): boolean {
  return BOMB_KINDS.has(kind);
}

/** What the timeline's tick of an event answers to: kills, grenades, and the rest (the bomb). */
export function timelineTickShown(kind: TacticalEventKind, show: { tlEvents: boolean; tlUtility: boolean; tlKills: boolean }): boolean {
  if (kind === TACTICAL_EVENT_KINDS.kill) return show.tlKills;
  if (isUtilityKind(kind)) return show.tlUtility;
  return show.tlEvents;
}

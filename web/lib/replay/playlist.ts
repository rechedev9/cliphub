import type { ReplayRef } from '../analysis/replay-link.ts';

/** Rounds the multi-round view superposes at most; more turns the map into noise. */
export const MULTI_MAX_ROUNDS = 10;

/** One stable key per strip item; the same round twice with different ticks stays two items. */
export function entryKey(entry: ReplayRef): string {
  return `${entry.matchId}:${entry.round}:${entry.tick ?? ''}`;
}

/** The index `delta` items away, held inside the strip instead of wrapping. */
export function stepIndex(length: number, index: number, delta: number): number {
  if (length <= 0) return 0;
  return Math.min(length - 1, Math.max(0, index + delta));
}

/** Matches whose documents the strip needs now: the current item and the next `ahead`, in order. */
export function matchesToLoad(entries: readonly ReplayRef[], index: number, ahead: number): string[] {
  const out: string[] = [];
  for (let i = Math.max(0, index); i <= index + ahead && i < entries.length; i++) {
    const id = entries[i].matchId;
    if (!out.includes(id)) out.push(id);
  }
  return out;
}

/** A run of consecutive strip items from one match, `end` exclusive. */
export type StripGroup = { matchId: string; start: number; end: number };

/** Splits the strip into runs of one match, so each run is labelled once. */
export function stripGroups(entries: readonly ReplayRef[]): StripGroup[] {
  const groups: StripGroup[] = [];
  entries.forEach((entry, index) => {
    const last = groups.at(-1);
    if (last !== undefined && last.matchId === entry.matchId) last.end = index + 1;
    else groups.push({ matchId: entry.matchId, start: index, end: index + 1 });
  });
  return groups;
}

/** Where the strip lands after it changes: on the same item when it survived, else the first. */
export function indexOfKey(entries: readonly ReplayRef[], key: string | null): number {
  if (key === null) return 0;
  const found = entries.findIndex((entry) => entryKey(entry) === key);
  return found < 0 ? 0 : found;
}

/**
 * The rounds the multi-round view opens with: the focused one, then its
 * neighbours from the same match, after it first, up to the cap.
 */
export function defaultMultiSelection(entries: readonly ReplayRef[], index: number, max: number = MULTI_MAX_ROUNDS): string[] {
  const focus = entries[index];
  if (focus === undefined) return [];
  const keys = [entryKey(focus)];
  const order = [...entries.slice(index + 1), ...entries.slice(0, index).reverse()];
  for (const entry of order) {
    if (keys.length >= max) break;
    if (entry.matchId === focus.matchId) keys.push(entryKey(entry));
  }
  return keys;
}

/** Adds or removes one round from the overlay; the focused round always stays and the cap holds. */
export function toggleMultiKey(selected: readonly string[], key: string, focusKey: string, max: number = MULTI_MAX_ROUNDS): string[] {
  if (key === focusKey) return selected.includes(key) ? [...selected] : [key, ...selected];
  if (selected.includes(key)) return selected.filter((entry) => entry !== key);
  if (selected.length >= max) return [...selected];
  return [...selected, key];
}

/** The overlay with the focused round in it, whatever was picked before the focus moved. */
export function withFocus(selected: readonly string[], focusKey: string, max: number = MULTI_MAX_ROUNDS): string[] {
  if (focusKey === '' || selected.includes(focusKey)) return [...selected];
  return [focusKey, ...selected].slice(0, max);
}

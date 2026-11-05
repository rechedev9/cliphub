import { COLLECTION_ITEMS_MAX, COLLECTION_NOTE_MAX, sameMoment } from '../api/collections.ts';
import type { CollectionItem } from '../api/collections.ts';
import type { ReplayRef } from '../analysis/replay-link.ts';

/** What adding a moment did: added, updated the note of the same moment, or hit the cap. */
export type AddOutcome = 'added' | 'updated' | 'full';

/**
 * Appends a moment, the way the server keeps it: the same match, round and
 * tick saved twice stays one item, and a new note replaces the old one.
 */
export function addMoment(items: readonly CollectionItem[], item: CollectionItem): { items: CollectionItem[]; outcome: AddOutcome } {
  const note = clampNote(item.note);
  const existing = items.findIndex((entry) => sameMoment(entry, item));
  if (existing >= 0) {
    const next = [...items];
    if (note !== '') next[existing] = { ...next[existing], note };
    return { items: next, outcome: 'updated' };
  }
  if (items.length >= COLLECTION_ITEMS_MAX) return { items: [...items], outcome: 'full' };
  return { items: [...items, { ...item, note }], outcome: 'added' };
}

/** Moves one item up (-1) or down (+1); out-of-range moves leave the list as it was. */
export function moveItem(items: readonly CollectionItem[], index: number, delta: number): CollectionItem[] {
  const target = index + delta;
  if (index < 0 || index >= items.length || target < 0 || target >= items.length) return [...items];
  const next = [...items];
  const [moved] = next.splice(index, 1);
  next.splice(target, 0, moved);
  return next;
}

export function removeItem(items: readonly CollectionItem[], index: number): CollectionItem[] {
  return items.filter((_, i) => i !== index);
}

export function setItemNote(items: readonly CollectionItem[], index: number, note: string): CollectionItem[] {
  return items.map((item, i) => (i === index ? { ...item, note: clampNote(note) } : item));
}

/** Trimmed and cut to the server's limit, counted in characters like the server. */
export function clampNote(note: string): string {
  return [...note.trim()].slice(0, COLLECTION_NOTE_MAX).join('');
}

/** The collection as a replay playlist, in its own order, without the moments whose match is gone. */
export function collectionRefs(items: readonly CollectionItem[], missing: readonly string[] = []): ReplayRef[] {
  const gone = new Set(missing);
  return items
    .filter((item) => !gone.has(item.match_id))
    .map((item) => ({ matchId: item.match_id, round: item.round, tick: item.tick }));
}

/** How many of the items still have their match. */
export function playableCount(items: readonly CollectionItem[], missing: readonly string[]): number {
  const gone = new Set(missing);
  return items.filter((item) => !gone.has(item.match_id)).length;
}

/** `1 momento`, `3 momentos`. */
export function momentCountLabel(count: number): string {
  return `${count} ${count === 1 ? 'momento' : 'momentos'}`;
}

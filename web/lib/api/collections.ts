/** `library.MaxCollection*`: the server rejects anything past these. */
export const COLLECTION_NAME_MAX = 80;
export const COLLECTION_NOTE_MAX = 280;
export const COLLECTION_ITEMS_MAX = 200;

/** Mirrors `library.CollectionItem`: one saved tick of a round of a match. */
export type CollectionItem = { match_id: string; round: number; tick: number; note: string };

/** Mirrors `library.Collection`, a named list of moments from any matches. */
export type LibraryCollection = {
  id: string;
  name: string;
  items: CollectionItem[];
  created_at: string;
  updated_at: string;
};

export type LibraryCollectionDetail = LibraryCollection & { missing_match_ids: string[] };

export type CollectionInput = { name: string; items?: CollectionItem[] };
export type CollectionPatch = { name?: string; items?: CollectionItem[] };

export const COLLECTION_KEYS = ['id', 'name', 'items', 'created_at', 'updated_at'] as const;
export const COLLECTION_DETAIL_KEYS = [...COLLECTION_KEYS, 'missing_match_ids'] as const;
export const COLLECTION_FIELDS = ['name', 'items'] as const;

/** Same moment: notes do not count, like the server's dedupe. */
export function sameMoment(a: CollectionItem, b: CollectionItem): boolean {
  return a.match_id === b.match_id && a.round === b.round && a.tick === b.tick;
}

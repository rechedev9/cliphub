/** Where a Libreta entry stands (`library.Statuses`); archived entries are hidden from lists by default. */
export const LIBRETA_STATUSES = ['active', 'practice', 'draft', 'review', 'archived'] as const;
export type LibretaStatus = (typeof LIBRETA_STATUSES)[number];

export function isLibretaStatus(value: string): value is LibretaStatus {
  return LIBRETA_STATUSES.some((status) => status === value);
}

/** The server's caps on an entry's tags (`library.NormalizeTags`). */
export const MAX_TAGS = 32;
export const MAX_TAG_LENGTH = 40;

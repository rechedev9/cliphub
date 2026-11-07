import { MAX_TAG_LENGTH, MAX_TAGS, type LibretaStatus } from '../api/libreta.ts';

export const STATUS_LABEL: Record<LibretaStatus, string> = {
  active: 'En uso',
  practice: 'Para practicar',
  draft: 'Borrador',
  review: 'En revisión',
  archived: 'Archivo',
};

export type StatusTone = 'success' | 'warning' | 'secondary' | 'stream' | 'outline';

export const STATUS_TONE: Record<LibretaStatus, StatusTone> = {
  active: 'success',
  practice: 'warning',
  draft: 'secondary',
  review: 'stream',
  archived: 'outline',
};

/** Tags typed as "eco, rápida": trimmed, deduped without case, cut to the server's limits. */
export function parseTags(text: string): string[] {
  const seen = new Set<string>();
  const out: string[] = [];
  for (const raw of text.split(',')) {
    const tag = raw.split(/\s+/).filter(Boolean).join(' ').slice(0, MAX_TAG_LENGTH).trim();
    const key = tag.toLocaleLowerCase('es');
    if (tag === '' || seen.has(key)) continue;
    seen.add(key);
    out.push(tag);
    if (out.length === MAX_TAGS) break;
  }
  return out;
}

export function tagsText(tags: readonly string[]): string {
  return tags.join(', ');
}

/** Hides archived rows unless the status filter asks for them. */
export function visibleByStatus<Row extends { status: LibretaStatus }>(rows: readonly Row[], statusFilter: string, any: string): Row[] {
  return statusFilter === any ? rows.filter((row) => row.status !== 'archived') : rows.filter((row) => row.status === statusFilter);
}

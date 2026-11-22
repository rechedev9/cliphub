export type VocabChange = { heard: string; written: string };

type VocabPart = { text: string; heard: string | null };

export type VocabMark = { heard: string; parts: VocabPart[] };

type VocabMarkInput = {
  text: string;
  heard: string;
  vocabText: string | undefined;
  changes: readonly VocabChange[] | undefined;
  corrected: boolean;
};

type WordSearch = { tokens: readonly string[]; words: readonly string[]; from: number };

type SplitToken = { lead: string; core: string; trail: string };

function splitToken(token: string): SplitToken {
  const lead = token.match(/^[^\p{L}\p{N}]*/u)?.[0] ?? '';
  const rest = token.slice(lead.length);
  const trail = rest.match(/[^\p{L}\p{N}]*$/u)?.[0] ?? '';
  return { lead, core: rest.slice(0, rest.length - trail.length), trail };
}

function findWords({ tokens, words, from }: WordSearch): number {
  for (let start = from + (from % 2); start + 2 * (words.length - 1) < tokens.length; start += 2) {
    if (words.every((word, k) => splitToken(tokens[start + 2 * k] ?? '').core === word)) return start;
  }
  return -1;
}

function vocabParts(text: string, changes: readonly VocabChange[]): VocabPart[] {
  const tokens = text.split(/(\s+)/);
  const parts: VocabPart[] = [];
  let pending = '';
  const flush = (): void => {
    if (pending !== '') parts.push({ text: pending, heard: null });
    pending = '';
  };
  let cursor = 0;
  for (const change of changes) {
    const words = change.written.split(/\s+/).filter((word) => word !== '');
    const start = words.length === 0 ? -1 : findWords({ tokens, words, from: cursor });
    if (start < 0) continue;
    const end = start + 2 * (words.length - 1) + 1;
    const { lead } = splitToken(tokens[start] ?? '');
    const { trail } = splitToken(tokens[end - 1] ?? '');
    const marked = tokens.slice(start, end).join('');
    pending += tokens.slice(cursor, start).join('') + lead;
    flush();
    parts.push({ text: marked.slice(lead.length, marked.length - trail.length), heard: change.heard });
    pending = trail;
    cursor = end;
  }
  pending += tokens.slice(cursor).join('');
  flush();
  return parts;
}

export function vocabMark(input: VocabMarkInput): VocabMark | null {
  const { text, heard, vocabText, changes, corrected } = input;
  if (corrected || vocabText === undefined || vocabText === '' || text !== vocabText || changes === undefined || changes.length === 0) return null;
  return { heard, parts: vocabParts(text, changes) };
}

export function isVocabChange(value: unknown): value is VocabChange {
  return typeof value === 'object' && value !== null && 'heard' in value && 'written' in value
    && typeof value.heard === 'string' && typeof value.written === 'string';
}

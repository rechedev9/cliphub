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

function findWords(tokens: readonly string[], words: readonly string[], from: number): number {
  for (let start = from + (from % 2); start + 2 * (words.length - 1) < tokens.length; start += 2) {
    if (words.every((word, k) => tokens[start + 2 * k] === word)) return start;
  }
  return -1;
}

function vocabParts(text: string, changes: readonly VocabChange[]): VocabPart[] {
  const tokens = text.split(/(\s+)/);
  const parts: VocabPart[] = [];
  const plain = (chunk: string): void => {
    if (chunk !== '') parts.push({ text: chunk, heard: null });
  };
  let cursor = 0;
  for (const change of changes) {
    const words = change.written.split(/\s+/).filter((word) => word !== '');
    const start = words.length === 0 ? -1 : findWords(tokens, words, cursor);
    if (start < 0) continue;
    const end = start + 2 * (words.length - 1) + 1;
    plain(tokens.slice(cursor, start).join(''));
    parts.push({ text: tokens.slice(start, end).join(''), heard: change.heard });
    cursor = end;
  }
  plain(tokens.slice(cursor).join(''));
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

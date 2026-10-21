// The Markdown subset the algorithms guide uses: headings, paragraphs, flat lists, pipe tables,
// fenced code, rules, and the evidence marks ([R], [R*], [C], [I], [C, corregido], [no verificable]).

const EVIDENCE_LEVELS = ['R', 'R*', 'C', 'I'] as const;

export type EvidenceLevel = (typeof EVIDENCE_LEVELS)[number];

export type Inline =
  | { kind: 'text'; text: string }
  | { kind: 'strong'; children: Inline[] }
  | { kind: 'em'; children: Inline[] }
  | { kind: 'code'; text: string }
  | { kind: 'link'; href: string }
  | { kind: 'evidence'; level: EvidenceLevel; note: string | null }
  | { kind: 'caveat'; text: string };

export type Block =
  | { kind: 'heading'; level: 2 | 3; id: string; title: string; content: Inline[] }
  | { kind: 'paragraph'; content: Inline[] }
  | { kind: 'list'; start: number | null; items: Inline[][] }
  | { kind: 'table'; head: Inline[][]; rows: Inline[][][] }
  | { kind: 'code'; text: string }
  | { kind: 'rule' };

// Evidence precedes em so `[R*]` is never emphasis; em needs a non-word edge so `Q* ... P*` stays text.
const INLINE_RE =
  /`([^`]+)`|\[(R\*|R|C|I)(?:,\s*([^\]]+))?\]|\[(debilitado[^\]]*|no verificable)\]|\*\*(.+?)\*\*|(?<![\w*])\*(?![\s*])([^*\n]+?)(?<!\s)\*(?!\w)|(https?:\/\/[^\s<[]+)/g;

const URL_TRAILING_RE = /[.,:;!?"»\]]+$/;

function count(text: string, char: string): number {
  return text.split(char).length - 1;
}

/** Drops sentence punctuation after a URL, and a closing parenthesis it did not open. */
function trimUrl(raw: string): string {
  let url = raw.replace(URL_TRAILING_RE, '');
  while (url.endsWith(')') && count(url, ')') > count(url, '(')) url = url.slice(0, -1).replace(URL_TRAILING_RE, '');
  return url;
}

export function parseInline(source: string): Inline[] {
  const out: Inline[] = [];
  let last = 0;
  for (const match of source.matchAll(INLINE_RE)) {
    const index = match.index;
    if (index > last) out.push({ kind: 'text', text: source.slice(last, index) });
    const [whole, code, level, note, caveat, strong, em, url = ''] = match;
    const evidence = EVIDENCE_LEVELS.find((value) => value === level);
    let consumed = whole;
    if (code !== undefined) out.push({ kind: 'code', text: code });
    else if (evidence !== undefined) out.push({ kind: 'evidence', level: evidence, note: note?.trim() ?? null });
    else if (caveat !== undefined) out.push({ kind: 'caveat', text: caveat });
    else if (strong !== undefined) out.push({ kind: 'strong', children: parseInline(strong) });
    else if (em !== undefined) out.push({ kind: 'em', children: parseInline(em) });
    else {
      consumed = trimUrl(url);
      out.push({ kind: 'link', href: consumed });
    }
    last = index + consumed.length;
  }
  if (last < source.length) out.push({ kind: 'text', text: source.slice(last) });
  return out;
}

function plainText(content: Inline[]): string {
  return content
    .map((node) => {
      switch (node.kind) {
        case 'text':
        case 'code':
        case 'caveat':
          return node.text;
        case 'strong':
        case 'em':
          return plainText(node.children);
        case 'link':
          return node.href;
        case 'evidence':
          return node.note === null ? `[${node.level}]` : `[${node.level}, ${node.note}]`;
      }
    })
    .join('');
}

export function slugify(text: string): string {
  return text
    .normalize('NFD')
    .replace(/\p{Diacritic}/gu, '')
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/** Splits a table row on `|`, except inside a code span or when escaped as `\|`. */
function splitRow(line: string): string[] {
  const row = line.trim().replace(/^\|/, '').replace(/(?<!\\)\|$/, '');
  const cells: string[] = [];
  let cell = '';
  let inCode = false;
  for (let i = 0; i < row.length; i++) {
    const char = row[i];
    if (char === '\\' && row[i + 1] === '|') {
      cell += '|';
      i++;
    } else if (char === '|' && !inCode) {
      cells.push(cell.trim());
      cell = '';
    } else {
      if (char === '`') inCode = !inCode;
      cell += char;
    }
  }
  cells.push(cell.trim());
  return cells;
}

// The page header owns the h1, so `#` folds into level 2; deeper headings render as level 3.
const HEADING_RE = /^(#{1,6}) (.+)$/;
const ORDERED_RE = /^(\d+)\. (.*)$/;
const CONTINUATION_RE = /^\s+\S/;
const BULLET_RE = /^- (.*)$/;
const SEPARATOR_CELL_RE = /^:?-{3,}:?$/;

function isBlockStart(line: string): boolean {
  return line.startsWith('```') || line.startsWith('|') || line.trim() === '---' || HEADING_RE.test(line) || ORDERED_RE.test(line) || BULLET_RE.test(line);
}

export function parseMarkdown(source: string): Block[] {
  const lines = source.replace(/\r\n/g, '\n').split('\n');
  const blocks: Block[] = [];
  const usedIds = new Set<string>();
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];
    if (line.trim() === '') {
      i++;
      continue;
    }
    if (line.startsWith('```')) {
      const body: string[] = [];
      i++;
      while (i < lines.length && !lines[i].startsWith('```')) body.push(lines[i++]);
      i++;
      blocks.push({ kind: 'code', text: body.join('\n') });
      continue;
    }
    if (line.trim() === '---') {
      blocks.push({ kind: 'rule' });
      i++;
      continue;
    }
    const heading = HEADING_RE.exec(line);
    if (heading !== null) {
      const content = parseInline(heading[2]);
      const title = plainText(content);
      const base = slugify(title) || 'seccion';
      let id = base;
      for (let n = 2; usedIds.has(id); n++) id = `${base}-${n}`;
      usedIds.add(id);
      const level = heading[1].length <= 2 ? 2 : 3;
      blocks.push({ kind: 'heading', level, id, title, content });
      i++;
      continue;
    }
    if (line.startsWith('|')) {
      const rows: string[][] = [];
      while (i < lines.length && lines[i].startsWith('|')) {
        const cells = splitRow(lines[i]);
        if (!cells.every((cell) => SEPARATOR_CELL_RE.test(cell))) rows.push(cells);
        i++;
      }
      const [head = [], ...body] = rows;
      blocks.push({ kind: 'table', head: head.map(parseInline), rows: body.map((row) => row.map(parseInline)) });
      continue;
    }
    const first = ORDERED_RE.exec(line);
    if (first !== null || BULLET_RE.test(line)) {
      const itemRe = first === null ? BULLET_RE : ORDERED_RE;
      const items: string[] = [];
      while (i < lines.length) {
        const item = itemRe.exec(lines[i]);
        if (item !== null) items.push(item[item.length - 1]);
        else if (CONTINUATION_RE.test(lines[i])) items[items.length - 1] += ` ${lines[i].trim()}`;
        else break;
        i++;
      }
      blocks.push({ kind: 'list', start: first === null ? null : Number(first[1]), items: items.map(parseInline) });
      continue;
    }
    const text: string[] = [];
    while (i < lines.length && lines[i].trim() !== '' && !isBlockStart(lines[i])) text.push(lines[i++].trim());
    blocks.push({ kind: 'paragraph', content: parseInline(text.join(' ')) });
  }
  return blocks;
}

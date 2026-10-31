import test from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { parseInline, parseMarkdown, slugify, type Block, type Inline } from './markdown.ts';

const GUIDE = readFileSync(new URL('../../content/algoritmos.md', import.meta.url), 'utf8');

function textNodes(content: Inline[]): string[] {
  return content.flatMap((node) => {
    if (node.kind === 'text') return [node.text];
    if (node.kind === 'strong' || node.kind === 'em') return textNodes(node.children);
    return [];
  });
}

function inlinesOf(block: Block): Inline[][] {
  switch (block.kind) {
    case 'heading':
    case 'paragraph':
      return [block.content];
    case 'list':
      return block.items;
    case 'table':
      return [...block.head, ...block.rows.flat()];
    default:
      return [];
  }
}

test('markdown: emphasis needs a closing mark at a word edge', () => {
  assert.deepEqual(parseInline('**Sin relleno.** *(nota)*'), [
    { kind: 'strong', children: [{ kind: 'text', text: 'Sin relleno.' }] },
    { kind: 'text', text: ' ' },
    { kind: 'em', children: [{ kind: 'text', text: '(nota)' }] },
  ]);
  assert.deepEqual(parseInline('Q* y P* (x)'), [{ kind: 'text', text: 'Q* y P* (x)' }]);
});

test('markdown: code spans are literal and URLs drop trailing punctuation', () => {
  assert.deepEqual(parseInline('`[10,20,30]` en https://a.example/x.'), [
    { kind: 'code', text: '[10,20,30]' },
    { kind: 'text', text: ' en ' },
    { kind: 'link', href: 'https://a.example/x' },
    { kind: 'text', text: '.' },
  ]);
});

test('markdown: blocks split on structure, not only on blank lines', () => {
  const blocks = parseMarkdown('## Uno\n**Papers**\n- a\n- b\n\n| A | B |\n|---|---|\n| 1 | 2 |\n```\nx | y\n```\n---\n## Uno');
  assert.deepEqual(
    blocks.map((block) => block.kind),
    ['heading', 'paragraph', 'list', 'table', 'code', 'rule', 'heading'],
  );
  const ids = blocks.flatMap((block) => (block.kind === 'heading' ? [block.id] : []));
  assert.deepEqual(ids, ['uno', 'uno-2']);
});

test('markdown: URLs keep balanced parentheses and query commas', () => {
  assert.deepEqual(parseInline('(ver https://w.example/Foo_(bar)).'), [
    { kind: 'text', text: '(ver ' },
    { kind: 'link', href: 'https://w.example/Foo_(bar)' },
    { kind: 'text', text: ').' },
  ]);
  assert.deepEqual(parseInline('https://a.example/?a=1,2;'), [
    { kind: 'link', href: 'https://a.example/?a=1,2' },
    { kind: 'text', text: ';' },
  ]);
});

test('markdown: table cells keep pipes inside code and escaped pipes', () => {
  const [table] = parseMarkdown('| A | B |\n|---|---|\n| `a|b` | c \\| d |');
  assert.ok(table.kind === 'table');
  assert.deepEqual(table.rows[0], [[{ kind: 'code', text: 'a|b' }], [{ kind: 'text', text: 'c | d' }]]);
});

test('markdown: only an all-dash row is a table separator', () => {
  const [table] = parseMarkdown('| A | B |\n|:---|---:|\n| --- | n/d |');
  assert.ok(table.kind === 'table');
  assert.deepEqual(table.rows, [[[{ kind: 'text', text: '---' }], [{ kind: 'text', text: 'n/d' }]]]);
});

test('markdown: lists keep their start number and indented continuations', () => {
  const [list] = parseMarkdown('3. tres\n   sigue\n4. cuatro');
  assert.deepEqual(list, {
    kind: 'list',
    start: 3,
    items: [[{ kind: 'text', text: 'tres sigue' }], [{ kind: 'text', text: 'cuatro' }]],
  });
});

test('markdown: deep headings, spaced rules and colliding ids', () => {
  const blocks = parseMarkdown('# Alto\n#### Hondo\n---  \n## Uno 2\n## Uno\n## Uno\n## ¿?');
  assert.deepEqual(
    blocks.map((block) => (block.kind === 'heading' ? `${block.level}:${block.id}` : block.kind)),
    ['2:alto', '3:hondo', 'rule', '2:uno-2', '2:uno', '2:uno-3', '2:seccion'],
  );
});

test('markdown: slugs drop accents', () => {
  assert.equal(slugify('6. Cambios 2024-2026 que importan'), '6-cambios-2024-2026-que-importan');
  assert.equal(slugify('Qué mide el cliente'), 'que-mide-el-cliente');
});

test('markdown: the guide leaves no raw markup in its text', () => {
  const blocks = parseMarkdown(GUIDE);
  const leftovers = blocks
    .flatMap(inlinesOf)
    .flatMap(textNodes)
    .filter((text) => /\*|`|^#|\s---\s/.test(text));
  assert.deepEqual(leftovers, []);
});

test('markdown: every guide table row has as many cells as its header', () => {
  for (const block of parseMarkdown(GUIDE)) {
    if (block.kind !== 'table') continue;
    for (const row of block.rows) assert.equal(row.length, block.head.length);
  }
});

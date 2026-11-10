import test from 'node:test';
import assert from 'node:assert/strict';
import { parseInline } from '../algorithms/markdown.ts';
import { notesHtml } from './notes-html.ts';

const ORIGIN = 'http://127.0.0.1:3000';

test('a Studio path link becomes a route; other targets stay text', () => {
  assert.deepEqual(parseInline('[Split **B**](/replay?r=a:3&t=x) ya'), [
    { kind: 'route', href: '/replay?r=a:3&t=x', children: [{ kind: 'text', text: 'Split ' }, { kind: 'strong', children: [{ kind: 'text', text: 'B' }] }] },
    { kind: 'text', text: ' ya' },
  ]);
  assert.deepEqual(parseInline('[api](/api/library/reports)'), [{ kind: 'text', text: '[api](/api/library/reports)' }]);
  assert.deepEqual(parseInline('[fuera](https://evil.example)'), [
    { kind: 'text', text: '[fuera](' },
    { kind: 'link', href: 'https://evil.example' },
    { kind: 'text', text: ')' },
  ]);
});

test('the exported notes are Markdown with Studio links on the export origin', () => {
  const html = notesHtml('## Mirage\n\n- Ejecutan **B** <b>\n- [Ver splits](/replay?r=a:3)\n\nhttps://evil.example', ORIGIN);
  assert.ok(html.includes('<h3>Mirage</h3>'));
  assert.ok(html.includes('<li>Ejecutan <strong>B</strong> &lt;b&gt;</li>'));
  assert.ok(html.includes(`<a href="${ORIGIN}/replay?r=a:3">Ver splits</a>`));
  assert.equal(html.includes('href="https://evil.example"'), false);
  assert.equal(notesHtml('[Ver](/replay?r=a:3)', '').includes('href'), false);
});

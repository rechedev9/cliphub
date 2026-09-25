import test from 'node:test';
import assert from 'node:assert/strict';
import { escapeHtml, psQuote } from './escaping.ts';

test('escapeHtml escapes HTML-sensitive characters exactly once', () => {
  const cases: Array<{ name: string; input: unknown; want: string }> = [
    { name: 'all five HTML-sensitive characters', input: '&<>"\'', want: '&amp;&lt;&gt;&quot;&#39;' },
    // The & rule runs first so a literal < becomes &lt;, not &amp;lt;.
    { name: 'tag without double-escaping', input: '<script>', want: '&lt;script&gt;' },
    { name: 'bare ampersand', input: 'a & b', want: 'a &amp; b' },
    { name: 'plain text untouched', input: 'hola mundo 123', want: 'hola mundo 123' },
    { name: 'number stringified', input: 42, want: '42' },
    { name: 'undefined stringified', input: undefined, want: 'undefined' },
    { name: 'error stringified then escaped', input: new Error('a<b'), want: 'Error: a&lt;b' },
  ];
  for (const tc of cases) {
    assert.equal(escapeHtml(tc.input), tc.want, tc.name);
  }
});

test('psQuote wraps in single quotes and doubles embedded quotes', () => {
  const cases: Array<{ name: string; input: string; want: string }> = [
    { name: 'plain word', input: 'plain', want: "'plain'" },
    { name: 'embedded quote', input: "it's", want: "'it''s'" },
    { name: 'empty string', input: '', want: "''" },
    { name: 'path without quotes', input: 'C:\\Users\\me\\tools', want: "'C:\\Users\\me\\tools'" },
  ];
  for (const tc of cases) {
    assert.equal(psQuote(tc.input), tc.want, tc.name);
  }
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { lastLines } from './log-tail.ts';

test('lastLines keeps the trailing window of a log', () => {
  const cases: Array<{ name: string; text: string; count: number; want: string }> = [
    { name: 'fewer lines than the limit', text: 'a\nb\nc', count: 40, want: 'a\nb\nc' },
    { name: 'more lines than the limit', text: '1\n2\n3\n4\n5', count: 2, want: '4\n5' },
    { name: 'count equals the number of lines', text: 'x\ny\nz', count: 3, want: 'x\ny\nz' },
    { name: 'single line', text: 'only', count: 40, want: 'only' },
    { name: 'blank lines inside the window', text: 'a\n\nb\n\nc', count: 3, want: 'b\n\nc' },
  ];
  for (const tc of cases) {
    assert.equal(lastLines(tc.text, tc.count), tc.want, tc.name);
  }
});

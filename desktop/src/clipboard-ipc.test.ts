import assert from 'node:assert/strict';
import test from 'node:test';
import { parseClipboardWriteRequest } from './clipboard-ipc.ts';

test('accepts one bounded clipboard text field', () => {
  assert.deepEqual(parseClipboardWriteRequest({ text: 'FragForge' }), { text: 'FragForge' });
  assert.deepEqual(parseClipboardWriteRequest({ text: '' }), { text: '' });
});

test('rejects malformed and oversized clipboard requests', () => {
  for (const value of [
    null,
    [],
    'text',
    {},
    { text: 42 },
    { text: 'ok', extra: true },
    { text: 'x'.repeat(512 * 1024 + 1) },
  ]) {
    assert.throws(() => parseClipboardWriteRequest(value));
  }
});

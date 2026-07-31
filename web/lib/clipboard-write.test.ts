import assert from 'node:assert/strict';
import test from 'node:test';
import { writeClipboardText } from './clipboard-write.ts';

test('prefers the desktop clipboard bridge', async () => {
  const calls: string[] = [];
  await writeClipboardText('desktop', {
    tickcutClipboard: { writeText: async (value: string) => { calls.push(`bridge:${value}`); } },
    navigator: { clipboard: { writeText: async (value: string) => { calls.push(`browser:${value}`); } } },
  });
  assert.deepEqual(calls, ['bridge:desktop']);
});

test('falls back to the browser clipboard API', async () => {
  const calls: string[] = [];
  await writeClipboardText('browser', {
    navigator: { clipboard: { writeText: async (value: string) => { calls.push(value); } } },
  });
  assert.deepEqual(calls, ['browser']);
});

test('rejects when no clipboard surface exists', async () => {
  await assert.rejects(writeClipboardText('missing', {}), /unavailable/);
});

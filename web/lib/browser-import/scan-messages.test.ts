import assert from 'node:assert/strict';
import test from 'node:test';
import { parseProgressMessage, parseResultMessage } from './scan-messages.ts';

test('parseProgressMessage accepts a valid progress payload', () => {
  const message = { id: 'demo-1', type: 'progress', read: 4 * 1024 * 1024, total: 1024 * 1024 * 1024 };
  assert.deepEqual(parseProgressMessage(message), message);
});

test('parseProgressMessage rejects malformed payloads', () => {
  const malformed = [
    undefined,
    null,
    'progress',
    42,
    [],
    {},
    { id: 'demo-1', type: 'result', read: 1, total: 2 },
    { id: 'demo-1', type: 'progress', read: '1', total: 2 },
    { id: 'demo-1', type: 'progress', read: 1 },
    { id: 'demo-1', type: 'progress', read: 1, total: Number.NaN },
    { id: 7, type: 'progress', read: 1, total: 2 },
  ];
  for (const message of malformed) {
    assert.equal(parseProgressMessage(message), undefined, JSON.stringify(message));
  }
});

test('parseResultMessage accepts a success result', () => {
  const positions = new Uint8Array([1, 2, 3]);
  const message = {
    id: 'demo-1',
    type: 'result',
    result: {
      ok: true,
      sha256: 'abc123',
      map: 'de_mirage',
      rounds: 24,
      cs2Build: 14000,
      document: '{"schema_version":"1"}\n',
      positions,
      sizeBytes: 168_232_051,
      fileName: '1-abcd-1-2.dem',
    },
  };
  const parsed = parseResultMessage(message);
  assert.deepEqual(parsed, message);
  if (parsed?.result.ok) assert.equal(parsed.result.positions, positions);
});

test('parseResultMessage accepts a failure result', () => {
  const message = { id: 'demo-1', type: 'result', result: { ok: false, error: 'demo corrupta' } };
  assert.deepEqual(parseResultMessage(message), message);
});

test('parseResultMessage rejects malformed payloads', () => {
  const malformed = [
    undefined,
    null,
    {},
    { id: 'demo-1', type: 'progress', result: { ok: false, error: 'x' } },
    { id: 'demo-1', type: 'result' },
    { id: 'demo-1', type: 'result', result: null },
    { id: 'demo-1', type: 'result', result: { ok: true } },
    { id: 'demo-1', type: 'result', result: { ok: false } },
    { id: 'demo-1', type: 'result', result: { ok: 'yes', error: 'x' } },
    {
      id: 'demo-1',
      type: 'result',
      result: {
        ok: true,
        sha256: 'a',
        map: 'm',
        rounds: '1',
        cs2Build: 1,
        document: '{}',
        positions: new Uint8Array(),
      },
    },
    {
      id: 'demo-1',
      type: 'result',
      result: {
        ok: true,
        sha256: 'a',
        map: 'm',
        rounds: 1,
        cs2Build: 1,
        document: '{}',
        positions: 'not-bytes',
        sizeBytes: 1,
        fileName: 'a.dem',
      },
    },
    {
      id: 'demo-1',
      type: 'result',
      result: {
        ok: true,
        sha256: 'a',
        map: 'm',
        rounds: 1,
        cs2Build: 1,
        document: '{}',
        positions: new Uint8Array(),
        fileName: 'a.dem',
      },
    },
  ];
  for (const message of malformed) {
    assert.equal(parseResultMessage(message), undefined, JSON.stringify(message));
  }
});

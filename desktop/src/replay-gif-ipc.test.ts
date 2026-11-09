import assert from 'node:assert/strict';
import test from 'node:test';
import { GifTargets, parseReplayGifRequest } from './replay-gif-ipc.ts';

const gif = (extra = 4): Uint8Array => new Uint8Array([...'GIF89a'].map((char) => char.charCodeAt(0)).concat(Array.from({ length: extra }, () => 0)));

test('a pick names a bare .gif file the dialog can propose', () => {
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: 'Mirage FUT vs BetBoom R14 1-00 a 0-40.gif' }), {
    action: 'pick',
    fileName: 'Mirage FUT vs BetBoom R14 1-00 a 0-40.gif',
  });
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: '..\\..\\Windows\\evil' }), { action: 'pick', fileName: 'Windows evil.gif' });
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: '' }), { action: 'pick', fileName: 'replay.gif' });
});

test('a write carries a token and the bytes of a GIF, nothing else', () => {
  const bytes = gif();
  assert.deepEqual(parseReplayGifRequest({ action: 'write', token: 'abc', bytes }), { action: 'write', token: 'abc', bytes });
  for (const value of [
    null,
    { action: 'pick' },
    { action: 'pick', fileName: 'a', path: 'C:\\x.gif' },
    { action: 'write', token: '', bytes },
    { action: 'write', token: 'abc', bytes: [71, 73, 70] },
    { action: 'write', token: 'abc', bytes: new Uint8Array([1, 2, 3, 4, 5, 6, 7]) },
    { action: 'write', token: 'abc', bytes, path: 'C:\\x.gif' },
    { action: 'delete', token: 'abc' },
  ]) {
    assert.throws(() => parseReplayGifRequest(value));
  }
});

test('a picked path is handed back once, and not after it expires', () => {
  const targets = new GifTargets();
  const token = targets.add('C:\\Users\\me\\Pictures\\a.gif', 0);
  assert.equal(targets.take('other', 1), null);
  assert.equal(targets.take(token, 1), 'C:\\Users\\me\\Pictures\\a.gif');
  assert.equal(targets.take(token, 2), null);
  const late = targets.add('C:\\b.gif', 0);
  assert.equal(targets.take(late, 31 * 60 * 1000), null);
});

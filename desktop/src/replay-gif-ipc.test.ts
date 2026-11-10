import assert from 'node:assert/strict';
import test from 'node:test';
import { GifTargets, parseReplayGifRequest } from './replay-gif-ipc.ts';

const gif = (extra = 4): Uint8Array => new Uint8Array([...'GIF89a'].map((char) => char.charCodeAt(0)).concat(Array.from({ length: extra }, () => 0)));
const mp4 = (): Uint8Array => new Uint8Array([0, 0, 0, 0x20, ...[...'ftypisom'].map((char) => char.charCodeAt(0)), 0, 0]);

test('a pick names a bare .gif file the dialog can propose', () => {
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: 'Mirage FUT vs BetBoom R14 1-00 a 0-40.gif' }), {
    action: 'pick',
    fileName: 'Mirage FUT vs BetBoom R14 1-00 a 0-40.gif',
    format: 'gif',
  });
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: '..\\..\\Windows\\evil' }), { action: 'pick', fileName: 'Windows evil.gif', format: 'gif' });
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: '' }), { action: 'pick', fileName: 'replay.gif', format: 'gif' });
});

test('a video pick names a bare .mp4 file', () => {
  assert.deepEqual(parseReplayGifRequest({ action: 'pick', fileName: 'Mirage R14 1-00 a 0-40.mp4', format: 'mp4' }), {
    action: 'pick',
    fileName: 'Mirage R14 1-00 a 0-40.mp4',
    format: 'mp4',
  });
  assert.throws(() => parseReplayGifRequest({ action: 'pick', fileName: 'a', format: 'exe' }));
});

test('a write carries a token and the bytes of a GIF or an MP4, nothing else', () => {
  const bytes = gif();
  assert.deepEqual(parseReplayGifRequest({ action: 'write', token: 'abc', bytes }), { action: 'write', token: 'abc', bytes, format: 'gif' });
  const video = mp4();
  assert.deepEqual(parseReplayGifRequest({ action: 'write', token: 'abc', bytes: video }), { action: 'write', token: 'abc', bytes: video, format: 'mp4' });
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
  const token = targets.add({ path: 'C:\\Users\\me\\Pictures\\a.gif', format: 'gif' }, 0);
  assert.equal(targets.take('other', 1), null);
  assert.deepEqual(targets.take(token, 1), { path: 'C:\\Users\\me\\Pictures\\a.gif', format: 'gif' });
  assert.equal(targets.take(token, 2), null);
  const late = targets.add({ path: 'C:\\b.mp4', format: 'mp4' }, 0);
  assert.equal(targets.take(late, 31 * 60 * 1000), null);
});

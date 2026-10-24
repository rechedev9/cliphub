import test from 'node:test';
import assert from 'node:assert/strict';
import { StreamDownloads } from './stream-downloads.ts';
const origin = 'http://127.0.0.1:3210';
const url =
  origin +
  '/api/streams/f96beeb7-4829-4117-836f-13d3d8502430/renders/streamer-fullframe-nocam/videos/clip-1?v=revision1';
test('reveal keys accept only local Studio stream downloads, never paths or external URLs', () => {
  const downloads = new StreamDownloads();
  assert.equal(downloads.key(url, origin), url);
  for (const invalid of [
    'C:/Users/me/private.txt',
    'file:///C:/private.txt',
    url.replace(origin, 'https://evil.test'),
    origin + '/api/streams/../../private',
    url + '#fragment',
  ])
    assert.equal(downloads.key(invalid, origin), null);
  assert.equal(downloads.key(url, null), null);
});
test('only recorded completed files can be revealed and new revisions are independent', () => {
  const downloads = new StreamDownloads();
  assert.equal(downloads.savedPath(url), undefined);
  downloads.completed(url, 'C:/Downloads/short.mp4');
  assert.equal(downloads.savedPath(url), 'C:/Downloads/short.mp4');
  assert.equal(downloads.savedPath(url.replace('revision1', 'revision2')), undefined);
  downloads.completed(url, 'C:/Downloads/short (1).mp4');
  assert.equal(downloads.savedPath(url), 'C:/Downloads/short (1).mp4');
});

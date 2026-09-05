import assert from 'node:assert/strict';
import test from 'node:test';
import { mediaPlaybackKey, parseMediaPlaybackStore, updateMediaPlaybackStore } from './media-playback-store.ts';
import type { MediaPlaybackItem } from './api/playback.ts';

test('invalid playback storage is ignored', () => {
  assert.deepEqual(parseMediaPlaybackStore('{bad'), { version: 1, entries: {} });
  assert.deepEqual(parseMediaPlaybackStore('{"version":2,"entries":{}}'), { version: 1, entries: {} });
});

test('playback storage stays bounded to the 50 newest revisions', () => {
  let raw: string | null = null;
  for (let index = 0; index < 55; index += 1) {
    raw = updateMediaPlaybackStore(raw, `artifact-${index}`, {
      position: index,
      volume: 1,
      muted: false,
      loop: false,
      playbackRate: 1,
      updatedAt: index,
    });
  }
  const store = parseMediaPlaybackStore(raw);
  assert.equal(Object.keys(store.entries).length, 50);
  assert.equal(store.entries['artifact-0'], undefined);
  assert.equal(store.entries['artifact-54']?.position, 54);
});

test('legacy valid progress defaults listening speed to one', () => {
  const store = parseMediaPlaybackStore(JSON.stringify({
    version: 1,
    entries: { old: { position: 4, volume: 0.5, muted: false, loop: true, updatedAt: 2 } },
  }));
  assert.equal(store.entries.old?.playbackRate, 1);
});

test('playback storage bounds oversized and pre-existing entry sets on read', () => {
  assert.deepEqual(parseMediaPlaybackStore(`{"version":1,"entries":{},"padding":"${'x'.repeat(128 * 1024)}"}`), { version: 1, entries: {} });
  const entries = Object.fromEntries(Array.from({ length: 55 }, (_, index) => [
    `old-${index}`,
    { position: index, volume: 1, muted: false, loop: false, playbackRate: 1, updatedAt: index },
  ]));
  const parsed = parseMediaPlaybackStore(JSON.stringify({ version: 1, entries }));
  assert.equal(Object.keys(parsed.entries).length, 50);
  assert.equal(parsed.entries['old-0'], undefined);
  assert.equal(parsed.entries['old-54']?.position, 54);
});

test('a replacement artifact revision cannot reuse the previous progress key', () => {
  const item: MediaPlaybackItem = {
    id: 'demo:one',
    source: 'demo',
    jobId: 'job',
    variant: 'clean',
    artifactName: 'video.mp4',
    revision: 'revision-1',
    title: 'Video',
    format: '9:16',
    playbackUrl: '/api/demos/job/video',
    review: 'ready',
    warnings: [],
  };
  assert.notEqual(mediaPlaybackKey(item), mediaPlaybackKey({ ...item, revision: 'revision-2' }));
});

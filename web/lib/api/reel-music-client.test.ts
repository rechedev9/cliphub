import assert from 'node:assert/strict';
import test from 'node:test';
import { MockApiClient } from './mock.ts';

test('mock library music rerender queues a ready reel and rejects a no-op mix', async () => {
  const client = new MockApiClient();
  const ready = await client.getVideo('v-seed-ready');
  assert.ok(ready);
  assert.equal(ready.status, 'ready');

  await assert.rejects(() => client.rerenderVideoMusic(ready.id, {}), /unchanged/);
  await assert.rejects(
    () => client.rerenderVideoMusic('v-seed-rendering', { songId: 'song-tikitaka-1' }),
    /not ready/,
  );

  const withMusic = await client.rerenderVideoMusic(ready.id, {
    songId: 'song-tikitaka-1',
    musicVolume: 0.35,
  });
  assert.equal(withMusic.mode, 'music');
  assert.equal(withMusic.songId, 'song-tikitaka-1');
  assert.equal(withMusic.musicVolume, 0.35);
  assert.notEqual(withMusic.status, 'ready');
});

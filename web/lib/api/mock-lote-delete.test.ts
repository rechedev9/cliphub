import assert from 'node:assert/strict';
import test from 'node:test';
import { MockApiClient } from './mock.ts';
import { DEFAULT_EDIT_CONFIG } from './reel-store.ts';

test('the mock deletes one Short of a lote at a time and drops the lote with its last Short', async () => {
  const client = new MockApiClient();
  const [match] = await client.listMatches();
  assert.ok(match);
  const plays = (await client.findClips(match.id)).slice(0, 2).map((play) => play.id);
  assert.equal(plays.length, 2);
  const lote = await client.createVideo({ matchId: match.id, playIds: plays, mode: 'clean', editConfig: { ...DEFAULT_EDIT_CONFIG, separateShorts: true } });
  assert.match(lote.title, /^Auto-Shorts: 2 Shorts/);
  await client.deleteLoteShort(lote.id, plays[0] ?? '');
  assert.match((await client.getVideo(lote.id))?.title ?? '', /^Auto-Shorts: 1 Short\b/);
  await client.deleteLoteShort(lote.id, plays[0] ?? '');
  assert.match((await client.getVideo(lote.id))?.title ?? '', /^Auto-Shorts: 1 Short\b/);
  await client.deleteLoteShort(lote.id, plays[1] ?? '');
  assert.equal(await client.getVideo(lote.id), null);
});

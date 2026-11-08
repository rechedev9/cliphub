import test from 'node:test';
import assert from 'node:assert/strict';
import { proClipUrl, shownClip, type ProExample } from './pro-lineups.ts';

const example = (clip?: string): ProExample => ({
  match: '1',
  map_key: '1/a-m1-mirage.dem',
  event: 'Major',
  played_at: '2026-11-01T00:00:00Z',
  team: 'OG',
  player: 'spooke',
  round: 7,
  clock: 2,
  tick: 46593,
  ...(clip === undefined ? {} : { clip }),
});

test('a clip URL goes through the library proxy, one segment per part', () => {
  assert.equal(proClipUrl('1/a-m1-mirage-r7-t46593-76561198079183361.mp4'), '/api/library/pro-lineups/clips/1/a-m1-mirage-r7-t46593-76561198079183361.mp4');
});

test('the clip shown is the chosen recorded throw, else the lineup clip', () => {
  const recorded = example('1/a-r7-t1-76561198079183361.mp4');
  const other = example('1/a-r9-t2-76561198079183361.mp4');
  const examples = [example(), recorded, other];
  assert.equal(shownClip({ clip: recorded.clip, examples }, null)?.example, recorded);
  assert.equal(shownClip({ clip: recorded.clip, examples }, other.clip ?? null)?.example, other);
  assert.equal(shownClip({ examples: [example()] }, null), null);
});

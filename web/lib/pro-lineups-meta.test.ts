import assert from 'node:assert/strict';
import test from 'node:test';
import type { ProLineup } from './api/pro-lineups.ts';
import { grenadeMeta, metaTrend } from './pro-lineups-meta.ts';

function lineup(over: Partial<ProLineup>): ProLineup {
  return {
    id: 'x',
    slug: 'x',
    title: 'Humo',
    kind: 'smoke',
    side: 'T',
    throw_place: '',
    land_place: '',
    origin: [0, 0, 0],
    yaw: 0,
    pitch: 0,
    landing: [0, 0, 0],
    movement: 'standing',
    jump: false,
    insta: false,
    air_time: 0,
    throws: { recent: 0, all: 0 },
    matches: { recent: 0, all: 0 },
    round_share: 0,
    round_win: 0,
    attempts: 0,
    landed: 0,
    clock: { median: 0, buckets: [] },
    teams: [],
    players: [],
    events: [],
    plant_sites: [],
    with: [],
    examples: [],
    last_seen: '',
    team_count: 0,
    rank: { position: 1, of: 1 },
    ...over,
  };
}

test('a trend needs a real move to rise or fall, and a grenade new in the window enters the meta', () => {
  assert.equal(metaTrend([3, 0]), 'new');
  assert.equal(metaTrend([0.4, 0]), 'steady');
  assert.equal(metaTrend([0, 0]), 'steady');
  assert.equal(metaTrend([9, 6]), 'rising');
  assert.equal(metaTrend([1.2, 0.6]), 'steady');
  assert.equal(metaTrend([2, 6]), 'falling');
  assert.equal(metaTrend(undefined), null);
});

test('the meta ranks one side by throws per 100 side rounds of the period', () => {
  const data = {
    rounds_t: { recent: 200, all: 800 },
    rounds_ct: { recent: 100, all: 400 },
    lineups: [
      lineup({ id: 'old', throws: { recent: 10, all: 200 }, trend: [2, 6] }),
      lineup({ id: 'new', throws: { recent: 30, all: 30 }, trend: [12, 0] }),
      lineup({ id: 'flash', kind: 'flash', throws: { recent: 20, all: 60 } }),
      lineup({ id: 'gone', throws: { recent: 0, all: 40 } }),
      lineup({ id: 'ct', side: 'CT', throws: { recent: 50, all: 90 } }),
    ],
  };
  const recent = grenadeMeta(data, { side: 'T', kinds: new Set(['smoke', 'flash']), period: 'recent' });
  assert.deepEqual(
    recent.map((row) => [row.rank, row.lineup.id, row.per100, row.trend]),
    [
      [1, 'new', 15, 'new'],
      [2, 'flash', 10, null],
      [3, 'old', 5, 'falling'],
    ],
  );
  const all = grenadeMeta(data, { side: 'T', kinds: new Set(['smoke']), period: 'all' });
  assert.deepEqual(
    all.map((row) => [row.lineup.id, row.per100]),
    [
      ['old', 25],
      ['gone', 5],
      ['new', 3.8],
    ],
  );
});

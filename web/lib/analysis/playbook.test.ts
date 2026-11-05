import assert from 'node:assert/strict';
import test from 'node:test';
import type { PlaybookRound } from '../api/playbook.ts';
import {
  EMPTY_ZONE_FILTER,
  filterRoundsByZones,
  scaleBars,
  tempoBuckets,
  toggleZone,
  zoneFilterSentence,
  zoneRoundCounts,
} from './playbook.ts';

const MATCH = '11111111-1111-4111-8111-111111111111';

function round(number: number, overrides: Partial<PlaybookRound>): PlaybookRound {
  return {
    match_id: MATCH,
    round: number,
    tick: number * 1000,
    category: 'default',
    pattern: 'default',
    site: 'none',
    won: false,
    buy: 'full',
    opponent_buy: 'full',
    tempo_seconds: null,
    framed: true,
    zones: [],
    ...overrides,
  };
}

const hold = (zone: string): PlaybookRound['zones'][number] => ({ zone, from_seconds: 10, seconds: 6, players: 2 });

const ROUNDS = [
  round(1, { site: 'B', zones: [hold('TopofMid'), hold('Apartments')] }),
  round(2, { site: 'A', zones: [hold('TopofMid')] }),
  round(3, { site: 'B', zones: [hold('Apartments')] }),
  round(4, { site: 'B', zones: [] }),
];

test('zone chips combine: every chosen zone is held and the round ends on the chosen site', () => {
  const mid = toggleZone(EMPTY_ZONE_FILTER, 'TopofMid');
  assert.deepEqual(filterRoundsByZones(ROUNDS, mid).map((r) => r.round), [1, 2]);
  assert.deepEqual(filterRoundsByZones(ROUNDS, { ...mid, site: 'B' }).map((r) => r.round), [1]);
  assert.deepEqual(filterRoundsByZones(ROUNDS, toggleZone(mid, 'Apartments')).map((r) => r.round), [1]);
  assert.deepEqual(toggleZone(mid, 'TopofMid'), EMPTY_ZONE_FILTER);
  assert.equal(filterRoundsByZones(ROUNDS, EMPTY_ZONE_FILTER).length, 4);
});

test('chip counts read how many of the current rounds held each zone once', () => {
  const twice = round(5, { zones: [hold('TopofMid'), hold('TopofMid')] });
  const counts = zoneRoundCounts([...ROUNDS, twice]);
  assert.equal(counts.get('TopofMid'), 3);
  assert.equal(counts.get('Apartments'), 2);
});

test('the filter reads as one sentence', () => {
  assert.equal(zoneFilterSentence(['Mid'], 'B'), 'Controla Mid y acaba en B');
  assert.equal(zoneFilterSentence(['Mid', 'Banana', 'Ramp'], ''), 'Controla Mid, Banana y Ramp');
  assert.equal(zoneFilterSentence([], 'A'), 'Acaba en A');
  assert.equal(zoneFilterSentence([], ''), '');
});

test('tempo buckets keep empty gaps and skip rounds without a tempo', () => {
  const rows = [{ tempo_seconds: 2 }, { tempo_seconds: 12.5 }, { tempo_seconds: null }, { tempo_seconds: 14 }];
  const buckets = tempoBuckets(rows, 5);
  assert.deepEqual(buckets.map((bucket) => [bucket.fromSeconds, bucket.items.length]), [[0, 1], [5, 0], [10, 2]]);
  assert.deepEqual(tempoBuckets(rows, 0), []);
});

test('bars scale to the peak and a single round stays visible', () => {
  assert.deepEqual(scaleBars([0, 1, 50, 100]), [0, 4, 50, 100]);
  assert.deepEqual(scaleBars([0, 0]), [0, 0]);
});

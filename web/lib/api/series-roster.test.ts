// Unit tests for the pure series roster aggregation.
// Run: node --test series-roster.test.ts
import test from 'node:test';
import assert from 'node:assert/strict';
import { aggregateGroupedSeriesRoster, aggregateSeriesRoster } from './series-roster.ts';
import type { DemoPlayer } from './types.ts';

/** Builds a DemoPlayer with zeroed defaults so each case sets only what it tests. */
function mk(overrides: Partial<DemoPlayer> & { steamId: string }): DemoPlayer {
  return {
    steamId: overrides.steamId,
    name: overrides.name ?? 'player',
    team: overrides.team ?? 'CT',
    kills: overrides.kills ?? 0,
    deaths: overrides.deaths ?? 0,
    assists: overrides.assists ?? 0,
    headshots: overrides.headshots ?? 0,
    mvps: overrides.mvps ?? 0,
    rounds: overrides.rounds ?? 0,
    adr: overrides.adr ?? 0,
    hsPct: overrides.hsPct ?? 0,
    kast: overrides.kast ?? 0,
    rating: overrides.rating ?? 0,
    rounds2k: overrides.rounds2k,
    rounds3k: overrides.rounds3k,
    rounds4k: overrides.rounds4k,
    rounds5k: overrides.rounds5k,
  };
}

test('empty input yields no players', () => {
  assert.deepEqual(aggregateSeriesRoster([]), []);
  assert.deepEqual(aggregateSeriesRoster([[]]), []);
});

test('a single roster passes counts through and weights rates trivially', () => {
  const p = mk({
    steamId: '100', name: 'a', team: 'T',
    kills: 10, deaths: 5, assists: 2, headshots: 4, mvps: 1, rounds: 20,
    adr: 80, hsPct: 40, kast: 70, rating: 1.2, rounds2k: 2, rounds3k: 1,
  });
  const [agg] = aggregateSeriesRoster([[p]]);
  assert.equal(agg.mapsPresent, 1);
  assert.equal(agg.name, 'a');
  assert.equal(agg.team, 'T');
  assert.equal(agg.kills, 10);
  assert.equal(agg.deaths, 5);
  assert.equal(agg.assists, 2);
  assert.equal(agg.headshots, 4);
  assert.equal(agg.mvps, 1);
  assert.equal(agg.rounds, 20);
  assert.equal(agg.adr, 80);
  assert.equal(agg.hsPct, 40);
  assert.equal(agg.kast, 70);
  assert.equal(agg.rating, 1.2);
  assert.equal(agg.rounds2k, 2);
  assert.equal(agg.rounds3k, 1);
});

test('unions a player across maps and sums counting stats', () => {
  const map1 = mk({ steamId: '100', kills: 10, deaths: 5, assists: 2, headshots: 4, mvps: 1, rounds: 20, rounds2k: 1, rounds3k: 0, rounds4k: 0, rounds5k: 1 });
  const map2 = mk({ steamId: '100', kills: 6, deaths: 8, assists: 3, headshots: 2, mvps: 0, rounds: 24, rounds2k: 0, rounds3k: 1, rounds4k: 0, rounds5k: 0 });
  const [agg] = aggregateSeriesRoster([[map1], [map2]]);
  assert.equal(agg.mapsPresent, 2);
  assert.equal(agg.kills, 16);
  assert.equal(agg.deaths, 13);
  assert.equal(agg.assists, 5);
  assert.equal(agg.headshots, 6);
  assert.equal(agg.mvps, 1);
  assert.equal(agg.rounds, 44);
  assert.equal(agg.rounds2k, 1);
  assert.equal(agg.rounds3k, 1);
  assert.equal(agg.rounds5k, 1);
});

test('weights round-based rate stats and derives headshot percentage from total headshots and kills', () => {
  const map1 = mk({ steamId: '100', kills: 1, headshots: 1, rounds: 20, adr: 80, hsPct: 100, kast: 60, rating: 1.0 });
  const map2 = mk({ steamId: '100', kills: 20, headshots: 0, rounds: 24, adr: 100, hsPct: 0, kast: 80, rating: 1.4 });
  const [weighted] = aggregateSeriesRoster([[map1], [map2]]);
  assert.equal(weighted.adr, (80 * 20 + 100 * 24) / 44);
  assert.equal(weighted.hsPct, 100 / 21);
  assert.equal(weighted.kast, (60 * 20 + 80 * 24) / 44);
  assert.equal(weighted.rating, (1.0 * 20 + 1.4 * 24) / 44);

  // No rounds anywhere: fall back to a plain per-map average.
  const zero1 = mk({ steamId: '200', rounds: 0, adr: 60, rating: 1.1 });
  const zero2 = mk({ steamId: '200', rounds: 0, adr: 80, rating: 1.3 });
  const [plain] = aggregateSeriesRoster([[zero1], [zero2]]);
  assert.equal(plain.adr, (60 + 80) / 2);
  assert.equal(plain.rating, (1.1 + 1.3) / 2);
});

test('preserves reported headshot percentages when kill counts are unavailable', () => {
  const map1 = mk({ steamId: '200', kills: 0, headshots: 0, rounds: 10, hsPct: 60 });
  const map2 = mk({ steamId: '200', kills: 0, headshots: 0, rounds: 30, hsPct: 40 });
  const [weighted] = aggregateSeriesRoster([[map1], [map2]]);
  assert.equal(weighted.hsPct, 45);

  const zeroRounds1 = mk({ steamId: '300', kills: 0, headshots: 0, hsPct: 62.5 });
  const zeroRounds2 = mk({ steamId: '300', kills: 0, headshots: 0, hsPct: 37.5 });
  const [plain] = aggregateSeriesRoster([[zeroRounds1], [zeroRounds2]]);
  assert.equal(plain.hsPct, 50);
});

test('preserves every reported percentage when only some maps lack kill counts', () => {
  const counted = mk({
    steamId: '400',
    kills: 10,
    headshots: 5,
    rounds: 10,
    hsPct: 50,
  });
  const percentageOnly = mk({
    steamId: '400',
    kills: 0,
    headshots: 0,
    rounds: 30,
    hsPct: 30,
  });

  const [weighted] = aggregateSeriesRoster([[counted], [percentageOnly]]);

  assert.equal(weighted.hsPct, 35);
});

test('HLTV split parts count as one logical map and still sum their statistics', () => {
  const rows = [
    {
      jobId: 'p1',
      fileName: 'match-m1-inferno-p1.dem',
      players: [mk({ steamId: '100', kills: 4, headshots: 2, rounds: 10 })],
    },
    {
      jobId: 'p2',
      fileName: 'match-m1-inferno-p2.dem',
      players: [mk({ steamId: '100', kills: 6, headshots: 3, rounds: 12 })],
    },
    {
      jobId: 'm2',
      fileName: 'match-m2-mirage.dem',
      players: [mk({ steamId: '100', kills: 8, headshots: 4, rounds: 20 })],
    },
  ];
  const [player] = aggregateGroupedSeriesRoster(rows);
  assert.equal(player.mapsPresent, 2);
  assert.equal(player.kills, 18);
  assert.equal(player.headshots, 9);
  assert.equal(player.hsPct, 50);
});

test('HLTV split-part aggregation preserves hsPct-only coverage through later maps', () => {
  const rows = [
    {
      jobId: 'p1',
      fileName: 'match-m1-inferno-p1.dem',
      players: [mk({
        steamId: '100',
        kills: 10,
        headshots: 5,
        rounds: 10,
        hsPct: 50,
      })],
    },
    {
      jobId: 'p2',
      fileName: 'match-m1-inferno-p2.dem',
      players: [mk({
        steamId: '100',
        kills: 0,
        headshots: 0,
        rounds: 30,
        hsPct: 30,
      })],
    },
    {
      jobId: 'm2',
      fileName: 'match-m2-mirage.dem',
      players: [mk({
        steamId: '100',
        kills: 10,
        headshots: 10,
        rounds: 20,
        hsPct: 100,
      })],
    },
  ];

  const [player] = aggregateGroupedSeriesRoster(rows);

  assert.equal(player.mapsPresent, 2);
  assert.equal(player.hsPct, 170 / 3);
});

test('a player missing from a map only aggregates the maps they played', () => {
  const a1 = mk({ steamId: '100', name: 'a', kills: 8, rounds: 20, adr: 90 });
  const b1 = mk({ steamId: '200', name: 'b', kills: 4, rounds: 20, adr: 50 });
  const a2 = mk({ steamId: '100', name: 'a', kills: 5, rounds: 24, adr: 70 });
  const result = aggregateSeriesRoster([[a1, b1], [a2]]);
  const a = result.find((p) => p.steamId === '100');
  const b = result.find((p) => p.steamId === '200');
  assert.ok(a && b);
  assert.equal(a.mapsPresent, 2);
  assert.equal(a.kills, 13);
  assert.equal(a.adr, (90 * 20 + 70 * 24) / 44);
  assert.equal(b.mapsPresent, 1);
  assert.equal(b.kills, 4);
  assert.equal(b.adr, 50);
});

test('sorts by total kills descending, then steamId for determinism', () => {
  const x = mk({ steamId: '200', kills: 5 });
  const y = mk({ steamId: '100', kills: 5 });
  const z = mk({ steamId: '300', kills: 9 });
  const result = aggregateSeriesRoster([[x, y, z]]);
  assert.deepEqual(result.map((p) => p.steamId), ['300', '100', '200']);
});

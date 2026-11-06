import assert from 'node:assert/strict';
import test from 'node:test';
import type { PrepMatchup, PrepTempo } from '../api/playbook.ts';
import type { TacticalBuyType, TacticalSide } from '../api/tactical.ts';
import { matchupGrid, matchupKey, placeLabels, prepWindowLabel, secondsLabel, spotLevel, tempoRows, windowMarkers } from './prep.ts';

test('CT markers keep the main spot, a strong second one, and fan out shared callouts', () => {
  const spot = (callout: string, pct: number) => ({ callout, share: { pct } });
  const players = [
    { windows: [{ spots: [spot('BombsiteA', 60), spot('Jungle', 25), spot('CTSpawn', 10)] }] },
    { windows: [{ spots: [spot('BombsiteA', 40), spot('Unknown', 30)] }] },
  ];
  const centers = new Map<string, readonly [number, number]>([
    ['BombsiteA', [100, 200]],
    ['Jungle', [50, 60]],
    ['CTSpawn', [0, 0]],
  ]);
  const markers = windowMarkers(players, { window: 0, centers });
  assert.deepEqual(
    markers.map((m) => [m.player, m.callout, m.main, m.slot, m.slots]),
    [
      [0, 'BombsiteA', true, 0, 2],
      [0, 'Jungle', false, 0, 1],
      [1, 'BombsiteA', true, 1, 2],
    ],
  );
  assert.deepEqual(windowMarkers(players, { window: 3, centers }), []);
});

test('CT markers stand at the spot position, on its own floor', () => {
  const players = [
    { windows: [{ spots: [{ callout: 'Ramp', share: { pct: 70 }, pos: [10, 20, -600] as const, floor: 'lower' }] }] },
    { windows: [{ spots: [{ callout: 'Ramp', share: { pct: 50 }, pos: [900, 20, 0] as const }] }] },
    { windows: [{ spots: [{ callout: 'Ramp', share: { pct: 50 }, pos: [950, 40, 0] as const }] }] },
  ];
  const centers = new Map<string, readonly [number, number]>([['Ramp', [0, 0]]]);
  const upper = windowMarkers(players, { window: 0, centers });
  assert.deepEqual(upper.map((m) => [m.player, m.x, m.y, m.slot, m.slots]), [
    [1, 900, 20, 0, 2],
    [2, 950, 40, 1, 2],
  ]);
  const lower = windowMarkers(players, { window: 0, centers, level: 'lower' });
  assert.deepEqual(lower.map((m) => [m.player, m.x, m.slots]), [[0, 10, 1]]);
  assert.equal(spotLevel({ floor: 'lower' }), 'lower');
  assert.equal(spotLevel({}), 'default');
});

test('labels of close dots move to a free side', () => {
  const dot = (x: number, y: number) => ({ x, y, width: 6, height: 2 });
  assert.deepEqual(placeLabels([dot(0, 0), dot(1, 0.5), dot(50, 50)], 1), ['right', 'left', 'right']);
});

const rate = (count: number, total: number): PrepMatchup['share'] => ({ count, total, pct: total > 0 ? (count / total) * 100 : 0, reliable: total >= 4 });

function matchup(side: TacticalSide, buy: TacticalBuyType, opponentBuy: TacticalBuyType, won: number, played: number): PrepMatchup {
  return { side, buy, opponent_buy: opponentBuy, rounds: played, share: rate(played, 40), win_rate: rate(won, played), examples: [], refs: [] };
}

test('prep windows read as spans, the last one open', () => {
  assert.equal(prepWindowLabel({ from_seconds: 20, to_seconds: 40 }), '20-40 s');
  assert.equal(prepWindowLabel({ from_seconds: 60, to_seconds: 0 }), '60 s o más');
});

test('a side matchup grid orders buys economically and drops unknown buys', () => {
  const grid = matchupGrid(
    [
      matchup('CT', 'full', 'eco', 9, 10),
      matchup('CT', 'eco', 'full', 1, 10),
      matchup('CT', 'unknown', 'full', 5, 5),
      matchup('T', 'full', 'full', 5, 10),
    ],
    'CT',
  );
  assert.deepEqual(grid.buys, ['eco', 'full']);
  assert.deepEqual(grid.opponentBuys, ['eco', 'full']);
  assert.equal(grid.cells.get(matchupKey('full', 'eco'))?.win_rate.count, 9);
  assert.equal(grid.baselinePct, 50);
  assert.equal(matchupGrid([], 'T').baselinePct, null);
});

test('tempo rows keep one side with samples, in economic order', () => {
  const histogram = (samples: number): PrepTempo['tempo'] => ({ bucket_seconds: 5, samples, median: 20, buckets: [] });
  const row = (side: TacticalSide, buy: TacticalBuyType, samples: number): PrepTempo => ({
    side, buy, rounds: samples, share: rate(samples, 30), win_rate: rate(0, samples), examples: [], refs: [], tempo: histogram(samples), plant: histogram(0),
  });
  const rows = tempoRows([row('T', 'full', 8), row('T', 'eco', 3), row('T', 'force', 0), row('CT', 'full', 9)], 'T');
  assert.deepEqual(rows.map((entry) => entry.buy), ['eco', 'full']);
});

test('seconds keep one decimal only under ten', () => {
  assert.equal(secondsLabel(7.25), '7,3 s');
  assert.equal(secondsLabel(23.6), '24 s');
});

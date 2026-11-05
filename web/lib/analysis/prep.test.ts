import assert from 'node:assert/strict';
import test from 'node:test';
import type { PrepMatchup, PrepTempo } from '../api/playbook.ts';
import type { TacticalBuyType, TacticalSide } from '../api/tactical.ts';
import { matchupGrid, matchupKey, prepWindowLabel, secondsLabel, tempoRows } from './prep.ts';

const rate = (count: number, total: number): PrepMatchup['share'] => ({ count, total, pct: total > 0 ? (count / total) * 100 : 0, reliable: total >= 4 });

function matchup(side: TacticalSide, buy: TacticalBuyType, opponentBuy: TacticalBuyType, won: number, played: number): PrepMatchup {
  return { side, buy, opponent_buy: opponentBuy, rounds: played, share: rate(played, 40), win_rate: rate(won, played), examples: [] };
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
    side, buy, rounds: samples, share: rate(samples, 30), win_rate: rate(0, samples), examples: [], tempo: histogram(samples), plant: histogram(0),
  });
  const rows = tempoRows([row('T', 'full', 8), row('T', 'eco', 3), row('T', 'force', 0), row('CT', 'full', 9)], 'T');
  assert.deepEqual(rows.map((entry) => entry.buy), ['eco', 'full']);
});

test('seconds keep one decimal only under ten', () => {
  assert.equal(secondsLabel(7.25), '7,3 s');
  assert.equal(secondsLabel(23.6), '24 s');
});

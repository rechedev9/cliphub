import test from 'node:test';
import assert from 'node:assert/strict';
import { BUCKET_SECONDS, binsInBox, clockHistogram, firstMovement, firstMovements, roundsThroughBox } from './pattern-canvas.ts';
import type { OverlayRound, OverlayTrack } from '../api/spatial.ts';
import type { RadarCalibration } from '../api/tactical.ts';

function track(overrides: Partial<OverlayTrack> & Pick<OverlayTrack, 'steamid64' | 'points'>): OverlayTrack {
  return { slot: 1, name: 'p', side: 'T', start: 0, died: false, ...overrides };
}

function round(tracks: OverlayTrack[], overrides: Partial<OverlayRound> = {}): OverlayRound {
  return {
    match_id: 'm1',
    round: 3,
    map: 'de_mirage',
    winner: 'T',
    t_pattern: 'default',
    ct_pattern: 'default',
    site: 'a',
    tick_freeze_end: 1000,
    tickrate: 64,
    duration_seconds: 100,
    tracks,
    events: [],
    ...overrides,
  } as OverlayRound;
}

test('the histogram buckets by the round clock, stacks by key and folds a late moment into the last bucket', () => {
  const items = [
    { s: 0, who: 'a' },
    { s: 9.9, who: 'a' },
    { s: 10, who: 'b' },
    { s: 112, who: 'b' },
    { s: 140, who: 'a' },
    { s: Number.NaN, who: 'a' },
  ];
  const { buckets, tallest } = clockHistogram({ items, secondsOf: (item) => item.s, keyOf: (item) => item.who });
  assert.equal(buckets.length, 12);
  assert.equal(BUCKET_SECONDS, 10);
  assert.equal(buckets[0]?.total, 2);
  assert.equal(buckets[1]?.byKey.get('b'), 1);
  assert.equal(buckets[11]?.total, 2);
  assert.equal(buckets[11]?.end, 115);
  assert.equal(tallest, 2);
});

test('the first movement is the first sample 64 units away from where the player started', () => {
  const still = track({ steamid64: '1', points: [[0, 0, 0], [10, 0, 0], [30, 0, 0], [70, 0, 0]], start: 2 });
  assert.equal(firstMovement(still, 0.5), 2.5);
  assert.equal(firstMovement(track({ steamid64: '1', points: [[0, 0, 0], [5, 5, 0]] }), 0.5), null);
  assert.equal(firstMovement(track({ steamid64: '1', points: [] }), 0.5), null);
});

test('first movements keep the chosen players and the window, and open the replay a little before', () => {
  const runner = track({ steamid64: '1', points: [[0, 0, 0], [100, 0, 0]], start: 20 });
  const lurker = track({ steamid64: '2', points: [[0, 0, 0], [100, 0, 0]], start: 80 });
  const rounds = [round([runner, lurker])];
  const all = firstMovements({ rounds, step: 0.5, players: [], tramo: { from: 0, to: 0 } });
  assert.deepEqual(all.map((row) => [row.steamid64, row.seconds]), [['1', 10.5], ['2', 40.5]]);
  const early = firstMovements({ rounds, step: 0.5, players: ['1', '2'], tramo: { from: 0, to: 30 } });
  assert.deepEqual(early.map((row) => row.steamid64), ['1']);
  assert.equal(early[0]?.tick, 1000 + Math.round(10.5 * 64) - 3 * 64);
  assert.equal(firstMovements({ rounds, step: 0.5, players: ['2'], tramo: { from: 0, to: 30 } }).length, 0);
});

test('a round goes through a box only inside the window and for a chosen player', () => {
  const box = { minX: 90, minY: -10, maxX: 200, maxY: 10 };
  const rounds = [round([track({ steamid64: '1', points: [[0, 0, 0], [100, 0, 0]], start: 40 })])];
  const base = { rounds, step: 0.5, box };
  assert.equal(roundsThroughBox({ ...base, players: [], tramo: { from: 0, to: 0 } }).length, 1);
  assert.equal(roundsThroughBox({ ...base, players: ['2'], tramo: { from: 0, to: 0 } }).length, 0);
  assert.equal(roundsThroughBox({ ...base, players: ['1'], tramo: { from: 0, to: 10 } }).length, 0);
  assert.equal(roundsThroughBox({ ...base, players: ['1'], tramo: { from: 15, to: 30 } }).length, 1);
});

test('heat bins keep the squares whose centre is inside the box', () => {
  const calibration: RadarCalibration = { map: 'de_mirage', source: 'overview', pos_x: 0, pos_y: 1024, scale: 1, size: 1024 };
  const bins: [number, number, number][] = [[10, 10, 4], [100, 100, 7], [63, 63, 1]];
  const kept = binsInBox({ bins, grid: 128, calibration, box: { minX: 0, minY: 512, maxX: 512, maxY: 1024 } });
  assert.deepEqual(kept, [[10, 10, 4], [63, 63, 1]]);
});

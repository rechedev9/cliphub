import test from 'node:test';
import assert from 'node:assert/strict';
import { LIVE_SAMPLE_SECONDS, liveHeat, trackLevels, tramoSpan } from './heat-live.ts';
import { trackPointAt } from './tactical-spatial.ts';
import type { LiveHeatInput } from './heat-live.ts';
import type { OverlayRound, OverlayTrack } from './api/spatial.ts';
import type { RadarCalibration } from './api/tactical.ts';

// A 1024-unit radar from (0, 1024) down to (1024, 0), so a grid of 8 has 128-unit squares.
const CALIBRATION: RadarCalibration = { map: 'de_test', source: 'overview', pos_x: 0, pos_y: 1024, scale: 1, size: 1024, lower_altitude_max: -100 };

function track(overrides: Partial<OverlayTrack>): OverlayTrack {
  return { slot: 0, steamid64: '1', name: 'a', side: 'CT', start: 0, points: [], died: false, ...overrides };
}

function round(tracks: OverlayTrack[], extra: Partial<OverlayRound> = {}): OverlayRound {
  return {
    match_id: 'm', round: 1, map: 'de_test', winner: 'CT', t_pattern: 'unknown', ct_pattern: 'unknown', site: 'unknown',
    tick_freeze_end: 0, tickrate: 64, duration_seconds: 60, tracks, events: [], ...extra,
  } as OverlayRound;
}

function input(rounds: OverlayRound[], overrides: Partial<LiveHeatInput> = {}): LiveHeatInput {
  return { rounds, step: 0.5, side: 'CT', kind: 'position', tramo: { from: 0, to: 0 }, calibration: CALIBRATION, grid: 8, cellSize: 64, ...overrides };
}

test('a track point interpolates its height and is undefined outside the track', () => {
  const t = track({ start: 2, points: [[0, 0, 0], [10, 20, 30]] });
  assert.deepEqual(trackPointAt(t, 1.25, 0.5), [5, 10, 15, 0]);
  assert.equal(trackPointAt(t, 0.5, 0.5), undefined);
  assert.equal(trackPointAt(t, 2, 0.5), undefined);
});

test('the span follows the tramo, the round end and the plant', () => {
  const planted = round([], { duration_seconds: 90, events: [{ seconds: 40, tick: 0, kind: 'plant', pos: [0, 0, 0], subject: false }] });
  assert.deepEqual(tramoSpan(planted, { from: 0, to: 0 }), { from: 0, to: 90 });
  assert.deepEqual(tramoSpan(planted, { from: 10, to: 30 }), { from: 10, to: 30 });
  assert.deepEqual(tramoSpan(planted, { from: 0, to: 0, fromPlant: true }), { from: 40, to: 90 });
  assert.equal(tramoSpan(round([]), { from: 0, to: 0, fromPlant: true }), null);
  assert.equal(tramoSpan(round([], { duration_seconds: 20 }), { from: 30, to: 50 }), null);
});

test('positions count one instant per sample of the tramo while the player lives, on their side only', () => {
  // Standing still in the square of column 1, row 6 for 10 s, then the track ends (death).
  const still = track({ points: Array.from({ length: 21 }, () => [200, 200, 0] as [number, number, number]) });
  const enemy = track({ side: 'T', points: [[900, 900, 0], [900, 900, 0]] });
  const heat = liveHeat(input([round([still, enemy])]));
  assert.ok(heat !== null);
  assert.equal(heat.layers.length, 1);
  assert.deepEqual(heat.layers[0].bins, [[1, 6, Math.floor(10 / LIVE_SAMPLE_SECONDS)]]);
  const early = liveHeat(input([round([still])], { tramo: { from: 0, to: 5 } }));
  assert.equal(early?.layers[0].bins[0][2], Math.floor(5 / LIVE_SAMPLE_SECONDS));
});

test('the lower floor gets its own layer', () => {
  const below = track({ points: [[200, 200, -300], [200, 200, -300], [200, 200, -300]] });
  const heat = liveHeat(input([round([below])]));
  assert.deepEqual(heat?.layers.map((layer) => layer.level), ['lower']);
});

test('a player crossing the cut on a ramp keeps the floor they came from inside the dead band', () => {
  // The cut is -100; -130 is within the 48-unit band, so the walker stays upstairs until -200.
  const ramp = track({ points: [[200, 200, 0], [200, 200, -130], [200, 200, -200]] });
  assert.deepEqual(trackLevels({ track: ramp, step: 0.5, tickrate: 64, calibration: CALIBRATION }), ['default', 'default', 'lower']);
});

test('timing counts the first arrival of each player at each cell, with its median seconds', () => {
  // Starts at (32, 32) at 0 s, walks to (200, 32) at 0.5 s and stays: two cells, arrived at 0 and 0.5 s.
  const walker = track({ points: [[32, 32, 0], [200, 32, 0], [200, 32, 0], [200, 32, 0]] });
  const late = track({ slot: 1, start: 4, points: [[200, 32, 0]] });
  const heat = liveHeat(input([round([walker, late])], { kind: 'timing' }));
  assert.ok(heat !== null);
  const layer = heat.layers[0];
  assert.deepEqual(layer.bins, [[0, 7, 1], [1, 7, 2]]);
  assert.deepEqual(layer.seconds, [0, 1.3]);
  assert.equal(heat.maxSeconds, 1.3);
});

test('an unusable calibration paints nothing', () => {
  assert.equal(liveHeat(input([], { calibration: { ...CALIBRATION, scale: 0 } })), null);
});

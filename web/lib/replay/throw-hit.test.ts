import test from 'node:test';
import assert from 'node:assert/strict';
import type { RadarCalibration } from '../api/tactical.ts';
import { worldToRendered } from '../tactical-transform.ts';
import type { UtilityVisual } from './three/utility-visuals.ts';
import { throwAt, throwMarkers } from './throw-hit.ts';

const calibration: RadarCalibration = { map: 'de_nuke', source: 'overview', pos_x: -3453, pos_y: 2887, scale: 7, size: 1024, lower_altitude_max: -495 };
const SIZE = 500;
const arc = { from: { x: 0, y: 0, z: 0 }, to: { x: 0, y: 0, z: 0 }, progress: 0.5, fade: 1 };
const visual = (index: number, from: { x: number; y: number; z: number } | null, extra: Partial<UtilityVisual> = {}): UtilityVisual => ({
  index,
  kind: 'smoke',
  at: { x: 100, y: 100, z: 0 },
  from,
  effect: null,
  arc: from === null ? null : arc,
  ...extra,
});
const high = { x: -1000, y: -500, z: 10 };
const low = { x: -900, y: -400, z: -700 };

test('a throw marker sits where the arc starts: the throw position through the pane transform', () => {
  const [marker] = throwMarkers({ visuals: [visual(3, high)], calibration, size: SIZE, level: 'default' });
  const at = worldToRendered(calibration, high.x, high.y, SIZE);
  assert.deepEqual(marker, { index: 3, x: at.x, y: at.y });
});

test('with one floor per pane a marker shows only in the pane of the thrower settled floor', () => {
  const visuals = [visual(0, high), visual(1, low)];
  const only = (level: 'default' | 'lower'): number[] => throwMarkers({ visuals, calibration, size: SIZE, level, offFloorAlpha: 0 }).map((marker) => marker.index);
  assert.deepEqual(only('default'), [0]);
  assert.deepEqual(only('lower'), [1]);
});

test('the settled floor of the thrower wins over the height of the throw', () => {
  const throwLevels = new Map<number, 'default' | 'lower'>([[0, 'lower']]);
  const lower = throwMarkers({ visuals: [visual(0, high)], calibration, size: SIZE, level: 'lower', offFloorAlpha: 0, throwLevels });
  const upper = throwMarkers({ visuals: [visual(0, high)], calibration, size: SIZE, level: 'default', offFloorAlpha: 0, throwLevels });
  assert.equal(lower.length, 1);
  assert.equal(upper.length, 0);
});

test('on one plate a throw from the other floor still has its faint marker', () => {
  assert.equal(throwMarkers({ visuals: [visual(1, low)], calibration, size: SIZE, level: 'default' }).length, 1);
});

test('no marker without a recorded throw or once the arc has faded out', () => {
  const faded = visual(2, high, { arc: { ...arc, fade: 0 } });
  assert.deepEqual(throwMarkers({ visuals: [visual(0, null), faded], calibration, size: SIZE, level: 'default' }), []);
});

test('throwAt takes the closest marker inside the reach and none outside it', () => {
  const markers = [
    { index: 4, x: 100, y: 100 },
    { index: 7, x: 108, y: 100 },
  ];
  assert.equal(throwAt({ markers, x: 105, y: 100, reach: 8 }), 7);
  assert.equal(throwAt({ markers, x: 101, y: 100, reach: 8 }), 4);
  assert.equal(throwAt({ markers, x: 140, y: 100, reach: 8 }), null);
  assert.equal(throwAt({ markers: [], x: 0, y: 0, reach: 8 }), null);
});

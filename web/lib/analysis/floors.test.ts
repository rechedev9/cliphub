import assert from 'node:assert/strict';
import test from 'node:test';
import { RADAR_CALIBRATION_SOURCES, RADAR_LEVELS } from '../api/tactical.ts';
import { OFF_FLOOR_ALPHA, floorAlpha, labelOnFloor, onShownFloor } from './floors.ts';

const NUKE = { map: 'de_nuke', source: RADAR_CALIBRATION_SOURCES.overview, pos_x: -3453, pos_y: 2887, scale: 7, size: 1024, lower_altitude_max: -495 };
const MIRAGE = { map: 'de_mirage', source: RADAR_CALIBRATION_SOURCES.overview, pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 };

test('floors: on the upper radar of a two-floor map, the lower floor recedes', () => {
  const upper = { calibration: NUKE, level: RADAR_LEVELS.default };
  assert.equal(floorAlpha(upper, -400), 1);
  assert.equal(floorAlpha(upper, -700), OFF_FLOOR_ALPHA);
  const lower = { calibration: NUKE, level: RADAR_LEVELS.lower };
  assert.equal(onShownFloor(lower, -700), true);
  assert.equal(floorAlpha(lower, -400), OFF_FLOOR_ALPHA);
});

test('floors: with both floors on screen, each pane hides the other floor', () => {
  const upperPane = { calibration: NUKE, level: RADAR_LEVELS.default, offFloorAlpha: 0 };
  assert.equal(floorAlpha(upperPane, -400), 1);
  assert.equal(floorAlpha(upperPane, -700), 0);
});

test('floors: a name label shows only on the plate of its own floor', () => {
  const upperPane = { calibration: NUKE, level: RADAR_LEVELS.default, offFloorAlpha: 0 };
  const lowerPane = { calibration: NUKE, level: RADAR_LEVELS.lower, offFloorAlpha: 0 };
  assert.equal(labelOnFloor(upperPane, -400), true);
  assert.equal(labelOnFloor(lowerPane, -400), false, 'an upper-floor player leaves no name on the lower plate');
  assert.equal(labelOnFloor(lowerPane, -700), true);
  assert.equal(labelOnFloor({ calibration: NUKE, level: RADAR_LEVELS.default }, -700), true, 'one floor at a time keeps the faded name');
});

test('floors: a one-floor map draws everything at full strength', () => {
  assert.equal(floorAlpha({ calibration: MIRAGE, level: RADAR_LEVELS.default }, -2000), 1);
});

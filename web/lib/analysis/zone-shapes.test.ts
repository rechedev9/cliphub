import assert from 'node:assert/strict';
import test from 'node:test';
import { GEOMETRY_SOURCE_OCCUPANCY, RADAR_CALIBRATION_SOURCES, RADAR_LEVELS, type TacticalGeometry } from '../api/tactical.ts';
import { zoneShapes } from './zone-shapes.ts';

const GEOMETRY: TacticalGeometry = {
  map: 'de_mirage',
  source: GEOMETRY_SOURCE_OCCUPANCY,
  calibration: { map: 'de_mirage', source: RADAR_CALIBRATION_SOURCES.overview, pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 },
  bounds: { min_x: -3000, min_y: -3000, max_x: 3000, max_y: 3000 },
  cell_size: 64,
  levels: [{ name: RADAR_LEVELS.default, cells: [[2, -3, 7]] }],
  callouts: [
    { name: 'TopofMid', level: RADAR_LEVELS.default, center: [-230, -1287], samples: 40, cells: [[2, -3]] },
    { name: 'Apartments', level: RADAR_LEVELS.default, center: [0, 0], samples: 10 },
  ],
  sample_count: 20,
};

test('a zone is drawn from its cells and labelled at its centre of play', () => {
  const [mid, apartments] = zoneShapes(GEOMETRY, ['TopofMid', 'Apartments', 'Unknown'], 100);
  // (-230 + 3230) / 5 = 600 px of 1024; (1713 + 1287) / 5 = 600 px.
  assert.equal(mid?.x, 58.59);
  assert.equal(mid?.y, 58.59);
  assert.match(mid?.path ?? '', /^M[\d.]+ [\d.]+h[\d.]+v[\d.]+h-[\d.]+z$/);
  assert.equal(apartments?.path, '');
  assert.equal(zoneShapes(GEOMETRY, ['Unknown']).length, 0);
});

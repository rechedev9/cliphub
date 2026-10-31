import test from 'node:test';
import assert from 'node:assert/strict';
import type { RadarCalibration } from './api/tactical.ts';
import type { Heatmap, OverlayTrack } from './api/spatial.ts';
import { worldToRendered } from './tactical-transform.ts';
import {
  arrowHead,
  elapsedLabel,
  heatStrength,
  heatmapCellAt,
  heatmapLayer,
  heatmapLevels,
  heatmapSide,
  nearestCallout,
  noteSeconds,
  noteVisibleAt,
  radarProjection,
  renderedToWorld,
  thinStroke,
  topCells,
  trackDeadAt,
  trackPositionAt,
  trackTrail,
} from './tactical-spatial.ts';

const MIRAGE: RadarCalibration = { map: 'de_mirage', source: 'overview', pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 };

test('the projection fits the view window into the box and centres it', () => {
  assert.deepEqual(radarProjection({ width: 800, height: 600 }, { x: 0, y: 0, width: 1, height: 1 }), { size: 600, offsetX: 100, offsetY: 0 });
  const cropped = radarProjection({ width: 500, height: 500 }, { x: 0.25, y: 0.25, width: 0.5, height: 0.5 });
  assert.deepEqual(cropped, { size: 1000, offsetX: -250, offsetY: -250 });
});

test('rendered to world inverts world to rendered at any size', () => {
  for (const size of [512, 1024, 1733]) {
    const point = worldToRendered(MIRAGE, -1200.5, 340, size);
    const back = renderedToWorld(MIRAGE, point.x, point.y, size);
    assert.ok(Math.abs(back.x + 1200.5) < 1e-6 && Math.abs(back.y - 340) < 1e-6, `size ${size}: ${JSON.stringify(back)}`);
  }
});

const HEATMAP: Heatmap = {
  kind: 'position',
  window: { from: 0, to: 0 },
  cell_size: 64,
  step_seconds: 1,
  example_limit: 3,
  matches: 1,
  rounds: 3,
  missing_positions: 0,
  sides: [
    { side: 'CT', level: 'default', total: 5, rounds: 2, max_count: 4 },
    { side: 'T', level: 'default', total: 9, rounds: 3, max_count: 9 },
    { side: 'T', level: 'lower', total: 1, rounds: 1, max_count: 1 },
  ],
  cells: [
    { side: 'CT', level: 'default', x: 0, y: 0, count: 4, rounds: 2, seconds: 12, examples: [] },
    { side: 'CT', level: 'default', x: -2, y: 3, count: 1, rounds: 1, seconds: 40, examples: [] },
    { side: 'T', level: 'default', x: 1, y: 1, count: 9, rounds: 3, seconds: 5, examples: [] },
    { side: 'T', level: 'lower', x: 1, y: 1, count: 1, rounds: 1, seconds: 30, examples: [] },
  ],
};

test('a heatmap layer is one side and level with its own scale', () => {
  const ct = heatmapLayer(HEATMAP, 'CT', 'default');
  assert.equal(ct.cells.length, 2);
  assert.equal(ct.maxCount, 4);
  assert.equal(ct.maxSeconds, 40);
  assert.equal(heatStrength('position', ct.cells[0], ct), 1);
  assert.equal(heatStrength('position', ct.cells[1], ct), 0.5);
  assert.ok(heatStrength('timing', ct.cells[0], ct) > heatStrength('timing', ct.cells[1], ct), 'earlier arrivals read stronger');
  assert.deepEqual(heatmapLevels(HEATMAP), ['default', 'lower']);
  assert.equal(heatmapSide(HEATMAP, ''), 'T');
  assert.equal(heatmapSide(HEATMAP, 'CT'), 'CT');
});

test('the cell under a world point uses the floor of each axis', () => {
  const ct = heatmapLayer(HEATMAP, 'CT', 'default');
  assert.equal(heatmapCellAt(ct, 64, 63.9, 0.1)?.count, 4);
  assert.equal(heatmapCellAt(ct, 64, -100, 200)?.count, 1);
  assert.equal(heatmapCellAt(ct, 64, 64, 0), undefined);
  assert.equal(heatmapCellAt(ct, 0, 1, 1), undefined);
});

const TRACK: OverlayTrack = {
  slot: 0,
  steamid64: '1',
  name: 'a',
  side: 'T',
  start: 2,
  points: [
    [0, 0, 0],
    [10, 0, 0],
    [10, 20, 0],
  ],
  died: true,
};

test('a track is placed on the shared clock and ends at its death', () => {
  assert.equal(trackPositionAt(TRACK, 0.5, 0.5), undefined);
  assert.deepEqual(trackPositionAt(TRACK, 1, 0.5), [0, 0]);
  assert.deepEqual(trackPositionAt(TRACK, 1.25, 0.5), [5, 0]);
  assert.deepEqual(trackPositionAt(TRACK, 2, 0.5), [10, 20]);
  assert.equal(trackPositionAt(TRACK, 2.1, 0.5), undefined);
  assert.equal(trackDeadAt(TRACK, 2, 0.5), false);
  assert.equal(trackDeadAt(TRACK, 2.1, 0.5), true);
  assert.deepEqual(trackTrail(TRACK, 1.75, 0.5, 10), [[0, 0], [10, 0], [10, 10]]);
  assert.deepEqual(trackTrail(TRACK, 1.75, 0.5, 0.4), [[10, 0], [10, 10]]);
});

test('notes are timed from their round freeze end and show near the playhead', () => {
  assert.equal(noteSeconds({ tick: 1280 }, { tick_freeze_end: 640, tickrate: 64 }), 10);
  assert.equal(noteSeconds({ tick: 1280 }, { tick_freeze_end: 640, tickrate: 0 }), 10);
  assert.equal(noteVisibleAt(10, 12.5), true);
  assert.equal(noteVisibleAt(10, 13.5), false);
});

test('a freehand stroke keeps its ends and drops points closer than the step', () => {
  const thinned = thinStroke([[0, 0], [1, 0], [2, 0], [12, 0], [13, 0]], 10);
  assert.deepEqual(thinned, [[0, 0], [12, 0], [13, 0]]);
  assert.deepEqual(thinStroke([[0, 0], [1, 1]], 10), [[0, 0], [1, 1]]);
});

test('labels and arrowheads', () => {
  assert.equal(elapsedLabel(75.9), '1:15');
  assert.equal(elapsedLabel(-3), '0:00');
  assert.equal(arrowHead({ x: 0, y: 0 }, { x: 10, y: 0 }, 5).split(' ')[0], '10.0,0.0');
});

test('the nearest callout prefers the same level and the busiest cells come first', () => {
  const callouts = [
    { name: 'Ramp', level: 'default', center: [100, 100] as [number, number] },
    { name: 'Vents', level: 'lower', center: [0, 0] as [number, number] },
  ];
  assert.equal(nearestCallout({ cell_size: 0, callouts }, 'default', 0, 0), 'Ramp');
  assert.equal(nearestCallout({ cell_size: 0, callouts }, 'lower', 90, 90), 'Vents');
  assert.equal(nearestCallout({ cell_size: 64, callouts: [] }, 'default', 0, 0), '');
  const top = topCells(heatmapLayer(HEATMAP, 'CT', 'default'), 1);
  assert.deepEqual(top.map((cell) => cell.count), [4]);
});

test('the nearest callout is the place that owns the cell, then its closest owned neighbour', () => {
  const geometry = {
    cell_size: 64,
    callouts: [
      // A long site whose far end lies closer to the stairs' centre than its own.
      { name: 'BombsiteA', level: 'default', center: [320, 32] as [number, number], cells: [[0, 0], [9, 0]] as [number, number][] },
      { name: 'Stairs', level: 'default', center: [700, 100] as [number, number], cells: [[10, 1]] as [number, number][] },
    ],
  };
  assert.equal(nearestCallout(geometry, 'default', 600, 40), 'BombsiteA');
  assert.equal(nearestCallout(geometry, 'default', 650, -30), 'BombsiteA');
  assert.equal(nearestCallout(geometry, 'default', 2000, 2000), 'Stairs');
  // Cells only answer for their own level; another level falls back to the centres.
  assert.equal(nearestCallout(geometry, 'lower', 600, 40), 'Stairs');
});

test('a cell shared by two floors names the place nearest in height, like the Go lookup', () => {
  const geometry = {
    cell_size: 64,
    callouts: [
      { name: 'Catwalk', level: 'default', center: [0, 0] as [number, number], cells: [[0, 0]] as [number, number][], stacked: [[0, 0, -168]] as [number, number, number][] },
      { name: 'Underpass', level: 'default', center: [0, 0] as [number, number], stacked: [[0, 0, -318]] as [number, number, number][] },
    ],
  };
  assert.equal(nearestCallout(geometry, 'default', 30, 30, -170), 'Catwalk');
  assert.equal(nearestCallout(geometry, 'default', 30, 30, -360), 'Underpass');
  // A heatmap cell carries no altitude: the cell's busiest place answers.
  assert.equal(nearestCallout(geometry, 'default', 30, 30), 'Catwalk');
});

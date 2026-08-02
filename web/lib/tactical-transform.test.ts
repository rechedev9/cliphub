// Unit tests for the world -> radar transform used by the tactical view.
// Run: pnpm --dir web run test:unit
import test from 'node:test';
import assert from 'node:assert/strict';
import {
  cellRadarRect,
  cellWorldCenter,
  cellWorldRect,
  geometryLevel,
  isCalibrationUsable,
  isMultiLevelMap,
  levelForAltitude,
  maxCellWeight,
  occupancyBounds,
  pixelToWorld,
  radarScaleFactor,
  radarViewRect,
  renderedToWorld,
  worldRectToRadarRect,
  worldToPixel,
  worldToRendered,
  yawToRadarAngle,
} from './tactical-transform.ts';
import type { RadarRect } from './tactical-transform.ts';
import {
  GEOMETRY_SOURCE_OCCUPANCY,
  RADAR_CALIBRATION_SOURCES,
  RADAR_LEVELS,
} from './api/tactical.ts';
import type {
  RadarBounds,
  RadarCalibration,
  TacticalGeometry,
  TacticalGeometryCell,
} from './api/tactical.ts';

/*
 * The calibrations are the ones CS2 ships in resource/overviews/<map>.txt, as
 * transcribed in internal/radarmap; the expected pixels are the same worked
 * values internal/radarmap/radarmap_test.go pins, so a sign or scale regression
 * fails on both sides of the boundary.
 */
function calibration(overrides: Partial<RadarCalibration> = {}): RadarCalibration {
  return {
    map: 'de_mirage',
    source: RADAR_CALIBRATION_SOURCES.overview,
    pos_x: -3230,
    pos_y: 1713,
    scale: 5,
    size: 1024,
    ...overrides,
  };
}

const MIRAGE = calibration();
const INFERNO = calibration({ map: 'de_inferno', pos_x: -2087, pos_y: 3870, scale: 4.9 });
const DUST2 = calibration({ map: 'de_dust2', pos_x: -2476, pos_y: 3239, scale: 4.4 });
const NUKE = calibration({
  map: 'de_nuke',
  pos_x: -3453,
  pos_y: 2887,
  scale: 7,
  lower_altitude_max: -495,
});

function assertClose(actual: number, expected: number, message: string): void {
  assert.ok(
    Math.abs(actual - expected) < 1e-9,
    `${message}: got ${actual}, want ${expected}`,
  );
}

function geometry(overrides: Partial<TacticalGeometry> = {}): TacticalGeometry {
  return {
    map: 'de_mirage',
    source: GEOMETRY_SOURCE_OCCUPANCY,
    calibration: MIRAGE,
    bounds: { min_x: -3000, min_y: -3000, max_x: 3000, max_y: 3000 },
    cell_size: 64,
    levels: [
      { name: RADAR_LEVELS.default, cells: [[2, -3, 7], [3, -3, 12], [4, 0, 1]] },
    ],
    callouts: [{ name: 'Mid', level: RADAR_LEVELS.default, center: [-500, -200], samples: 240 }],
    sample_count: 20,
    ...overrides,
  };
}

test('worldToPixel matches the CS2 overview transform, Y inverted', () => {
  const mid = worldToPixel(MIRAGE, -1500, -600);
  assertClose(mid.x, 346.0, 'mirage mid x');
  assertClose(mid.y, 462.6, 'mirage mid y');

  const corner = worldToPixel(MIRAGE, -3230, 1713);
  assertClose(corner.x, 0, 'mirage upper-left x');
  assertClose(corner.y, 0, 'mirage upper-left y');

  const banana = worldToPixel(INFERNO, 500, 1200);
  assertClose(banana.x, 527.9591836734694, 'inferno banana x');
  assertClose(banana.y, 544.8979591836735, 'inferno banana y');

  const origin = worldToPixel(DUST2, -2476, 3239);
  assertClose(origin.x, 0, 'dust2 origin x');
  assertClose(origin.y, 0, 'dust2 origin y');
});

test('a higher world Y is a smaller radar Y', () => {
  const north = worldToPixel(MIRAGE, 0, 1000);
  const south = worldToPixel(MIRAGE, 0, -1000);
  assert.ok(north.y < south.y);
  assertClose(south.y - north.y, 2000 / MIRAGE.scale, 'inverted Y span');
});

test('pixelToWorld round-trips worldToPixel', () => {
  for (const point of [
    [0, 0],
    [123.5, 987.25],
    [1023, 1023],
  ] as const) {
    const world = pixelToWorld(MIRAGE, point[0], point[1]);
    const back = worldToPixel(MIRAGE, world.x, world.y);
    assertClose(back.x, point[0], 'round-trip x');
    assertClose(back.y, point[1], 'round-trip y');
  }
});

test('scaling to a rendered size is one factor for both axes', () => {
  assert.equal(radarScaleFactor(MIRAGE, 1024), 1);
  assert.equal(radarScaleFactor(MIRAGE, 512), 0.5);
  // A 2048-pixel overview always halves its scale, so the same world point
  // lands on the same rendered pixel.
  const modern = calibration({ scale: 2.5, size: 2048 });
  const native = worldToRendered(MIRAGE, -1500, -600, 640);
  const doubled = worldToRendered(modern, -1500, -600, 640);
  assertClose(doubled.x, native.x, 'resolution-independent x');
  assertClose(doubled.y, native.y, 'resolution-independent y');

  const half = worldToRendered(MIRAGE, -1500, -600, 512);
  assertClose(half.x, 173, 'rendered x at half size');
  assertClose(half.y, 231.3, 'rendered y at half size');
});

test('renderedToWorld round-trips worldToRendered', () => {
  const rendered = worldToRendered(MIRAGE, -1500, -600, 720);
  const world = renderedToWorld(MIRAGE, rendered.x, rendered.y, 720);
  assertClose(world.x, -1500, 'rendered round-trip x');
  assertClose(world.y, -600, 'rendered round-trip y');
});

test('level selection splits on altitude and stays default without a lower section', () => {
  assert.ok(isMultiLevelMap(NUKE));
  assert.equal(levelForAltitude(NUKE, -600), RADAR_LEVELS.lower);
  // The threshold itself belongs to the lower level.
  assert.equal(levelForAltitude(NUKE, -495), RADAR_LEVELS.lower);
  assert.equal(levelForAltitude(NUKE, -100), RADAR_LEVELS.default);

  assert.equal(isMultiLevelMap(MIRAGE), false);
  assert.equal(levelForAltitude(MIRAGE, -99999), RADAR_LEVELS.default);
});

test('occupancy cells map onto the world rectangle they were binned from', () => {
  const geo = geometry();
  // Cells are indexed by floor(world / cell_size), so cell 2 spans [128, 192).
  assert.deepEqual(cellWorldRect(geo, 2, -3), { minX: 128, minY: -192, maxX: 192, maxY: -128 });
  assert.deepEqual(cellWorldCenter(geo, 2, -3), { x: 160, y: -160 });
  assert.deepEqual(cellWorldCenter(geo, 0, 0), { x: 32, y: 32 });
});

test('a cell rectangle is drawn from the world maximum Y downwards', () => {
  const geo = geometry();
  const rect = cellRadarRect(geo, [2, -3, 7], 1024);
  const top = worldToPixel(MIRAGE, 128, -128);
  assertClose(rect.x, top.x, 'cell rect x');
  assertClose(rect.y, top.y, 'cell rect y');
  // Positive extents: the Y inversion must not flip the rectangle inside out.
  assertClose(rect.width, geo.cell_size / MIRAGE.scale, 'cell rect width');
  assertClose(rect.height, geo.cell_size / MIRAGE.scale, 'cell rect height');
  assert.ok(rect.width > 0 && rect.height > 0);
});

test('a cell rectangle scales with the rendered size', () => {
  const geo = geometry();
  const full = cellRadarRect(geo, [4, 0, 1], 1024);
  const half = cellRadarRect(geo, [4, 0, 1], 512);
  assertClose(half.x, full.x / 2, 'scaled cell x');
  assertClose(half.y, full.y / 2, 'scaled cell y');
  assertClose(half.width, full.width / 2, 'scaled cell width');
});

test('a world rectangle keeps its corners on the matching radar corners', () => {
  const rect = worldRectToRadarRect(MIRAGE, { minX: -1000, minY: -1000, maxX: 0, maxY: 0 }, 1024);
  const topLeft = worldToPixel(MIRAGE, -1000, 0);
  assertClose(rect.x, topLeft.x, 'world rect x');
  assertClose(rect.y, topLeft.y, 'world rect y');
  assertClose(rect.width, 1000 / MIRAGE.scale, 'world rect width');
  assertClose(rect.height, 1000 / MIRAGE.scale, 'world rect height');
});

/**
 * A tight, heavy occupancy cluster (world x [-512, -320], y [256, 448]) and the
 * kind of stray sample — one lonely count far away — that used to unfold the
 * view back to the whole square.
 */
const CLUSTER_CELLS: readonly TacticalGeometryCell[] = [
  [-8, 4, 300],
  [-7, 4, 500],
  [-6, 5, 400],
  [-7, 6, 250],
];
const STRAY_CELL: TacticalGeometryCell = [30, -40, 1];

function occupancy(cells: readonly TacticalGeometryCell[]): TacticalGeometry {
  return geometry({ levels: [{ name: RADAR_LEVELS.default, cells: [...cells] }] });
}

/** Asserts `outer` contains `inner`; both are fractions of the same square. */
function assertContains(outer: RadarRect, inner: RadarRect, name: string): void {
  assert.ok(outer.x <= inner.x + 1e-9, `${name}: left edge`);
  assert.ok(outer.y <= inner.y + 1e-9, `${name}: top edge`);
  assert.ok(outer.x + outer.width >= inner.x + inner.width - 1e-9, `${name}: right edge`);
  assert.ok(outer.y + outer.height >= inner.y + inner.height - 1e-9, `${name}: bottom edge`);
}

test('occupancy bounds trim stray mass and are null without weighted cells', () => {
  const cases: readonly { name: string; geo: TacticalGeometry; want: RadarBounds | null }[] = [
    {
      name: 'heavy cells fix the span untouched',
      geo: occupancy(CLUSTER_CELLS),
      want: { min_x: -512, min_y: 256, max_x: -320, max_y: 448 },
    },
    {
      name: 'a stray one-sample cell is shed from both axes',
      geo: occupancy([...CLUSTER_CELLS, STRAY_CELL]),
      want: { min_x: -512, min_y: 256, max_x: -320, max_y: 448 },
    },
    { name: 'no levels means no occupancy to read', geo: geometry({ levels: [] }), want: null },
    {
      name: 'weightless cells mean no occupancy to read',
      geo: occupancy([[0, 0, 0]]),
      want: null,
    },
    { name: 'a zero cell size cannot place cells in the world', geo: geometry({ cell_size: 0 }), want: null },
  ];
  for (const row of cases) {
    assert.deepEqual(occupancyBounds(row.geo), row.want, row.name);
  }
});

test('the radar view window frames the play area without golden margins', () => {
  // Properties rather than literals: the margins are tuned by eye, so the
  // assertions pin what must hold of any tuning — the window frames the play
  // area, keeps a usable aspect, and ignores stray samples.
  const view = radarViewRect(occupancy(CLUSTER_CELLS));
  assert.ok(view.width < 1 && view.height < 1, 'a framed demo is a real crop');
  assertContains(
    view,
    worldRectToRadarRect(MIRAGE, { minX: -512, minY: 256, maxX: -320, maxY: 448 }, 1),
    'occupancy inside the window',
  );
  const aspect = view.width / view.height;
  assert.ok(aspect >= 3 / 4 - 1e-9 && aspect <= 16 / 9 + 1e-9, 'aspect stays in band');

  // The point of reading the grid: one stray sample no longer unfolds the
  // window back to the square, which is what a raw min/max bounds did.
  assert.deepEqual(
    radarViewRect(occupancy([...CLUSTER_CELLS, STRAY_CELL])),
    view,
    'a stray sample does not move the window',
  );

  // An elongated strip clamps at 16/9 by growing its short axis.
  const strip = Array.from({ length: 60 }, (_, i): TacticalGeometryCell => [i - 30, 0, 100]);
  const wide = radarViewRect(occupancy(strip));
  assertClose(wide.width / wide.height, 16 / 9, 'clamped aspect');

  // Without cells the raw server bounds still frame the window.
  const bounds = { min_x: -2000, min_y: -1000, max_x: 0, max_y: 1000 };
  const fallback = radarViewRect(geometry({ levels: [], bounds }));
  assert.ok(fallback.width < 1 && fallback.height < 1, 'framed fallback is a real crop');
  assertContains(
    fallback,
    worldRectToRadarRect(MIRAGE, { minX: -2000, minY: -1000, maxX: 0, maxY: 1000 }, 1),
    'bounds inside the window',
  );

  // The window is a viewport onto the square, so it can never leave it.
  for (const [name, rect] of [['occupancy', view], ['fallback', fallback], ['strip', wide]] as const) {
    assert.ok(rect.x >= 0 && rect.y >= 0, `${name}: origin inside the square`);
    assert.ok(
      rect.x + rect.width <= 1 + 1e-9 && rect.y + rect.height <= 1 + 1e-9,
      `${name}: window inside the square`,
    );
  }
});

test('the radar view window falls back to the whole square', () => {
  const cases: readonly { name: string; geo: TacticalGeometry }[] = [
    {
      name: 'empty bounds and no cells',
      geo: geometry({ levels: [], bounds: { min_x: 0, min_y: 0, max_x: 0, max_y: 0 } }),
    },
    { name: 'a zero cell size', geo: geometry({ cell_size: 0, levels: [] }) },
    { name: 'an unusable calibration', geo: geometry({ calibration: calibration({ scale: 0 }) }) },
  ];
  for (const row of cases) {
    assert.deepEqual(radarViewRect(row.geo), { x: 0, y: 0, width: 1, height: 1 }, row.name);
  }
});

test('geometry levels are looked up by name and weighted by their heaviest cell', () => {
  const geo = geometry();
  const level = geometryLevel(geo, RADAR_LEVELS.default);
  assert.ok(level);
  assert.equal(level.cells.length, 3);
  assert.equal(geometryLevel(geo, RADAR_LEVELS.lower), undefined);
  assert.equal(maxCellWeight(level), 12);
  assert.equal(maxCellWeight({ name: RADAR_LEVELS.lower, cells: [] }), 0);
});

test('yaw becomes a clockwise screen angle once Y is inverted', () => {
  assert.equal(yawToRadarAngle(0), 0);
  assert.equal(yawToRadarAngle(90), 270);
  assert.equal(yawToRadarAngle(270), 90);
  assert.equal(yawToRadarAngle(-90), 90);
  assert.equal(yawToRadarAngle(360), 0);
  assert.equal(yawToRadarAngle(Number.NaN), 0);
});

test('an unusable calibration is an error, never a silently infinite pixel', () => {
  const broken = calibration({ scale: 0 });
  assert.equal(isCalibrationUsable(broken), false);
  assert.equal(isCalibrationUsable(calibration({ size: 0 })), false);
  assert.ok(isCalibrationUsable(MIRAGE));
  assert.throws(() => worldToPixel(broken, 0, 0), /calibration for "de_mirage"/);
  assert.throws(() => worldToRendered(MIRAGE, 0, 0, 0), /rendered size 0 must be positive/);
  assert.throws(() => cellWorldRect(geometry({ cell_size: 0 }), 0, 0), /cell size 0 must be positive/);
});

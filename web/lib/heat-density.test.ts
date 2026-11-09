import test from 'node:test';
import assert from 'node:assert/strict';
import { DENSITY_THRESHOLDS, SMOOTHING_MAX, SMOOTHING_MIN, densityPicture, heatBandwidth, heatDensity } from './heat-density.ts';
import type { DensityPicture, HeatDensity } from './heat-density.ts';
import { heatRgb } from './tactical-spatial.ts';

const GRID = 150;

/** The pixel of a picture at the centre of a grid square, or undefined outside it. */
function pixel(picture: DensityPicture, column: number, row: number): number[] | undefined {
  const px = Math.floor((column + 0.5 - picture.x) * picture.scale);
  const py = Math.floor((row + 0.5 - picture.y) * picture.scale);
  const width = picture.width * picture.scale;
  if (px < 0 || py < 0 || px >= width || py >= picture.height * picture.scale) return undefined;
  const i = (py * width + px) * 4;
  return [...picture.rgba.slice(i, i + 4)];
}

function at(density: HeatDensity, column: number, row: number): number {
  return density.mass[row * density.grid + column];
}

test('the kernel width follows the map and the Suavizado slider, clamped', () => {
  assert.equal(heatBandwidth('de_nuke', 1), 1.4);
  assert.equal(heatBandwidth('de_inferno', 1), 2.1);
  assert.equal(heatBandwidth('de_dust2', 1), 2.4);
  assert.equal(heatBandwidth('de_ancient', 1), 2.6);
  assert.equal(heatBandwidth('de_unknown', 1), 2.2);
  assert.ok(Math.abs(heatBandwidth('de_mirage', 0.5) - 1.1) < 1e-9);
  assert.equal(heatBandwidth('de_mirage', 9), heatBandwidth('de_mirage', SMOOTHING_MAX));
  assert.equal(heatBandwidth('de_mirage', 0), heatBandwidth('de_mirage', SMOOTHING_MIN));
  assert.equal(heatBandwidth('de_mirage', Number.NaN), heatBandwidth('de_mirage', 1));
});

test('a bin spreads into an even bump that keeps its count', () => {
  const density = heatDensity({ grid: GRID, bins: [[70, 40, 10]], bandwidth: 2.2 });
  assert.ok(density);
  const total = density.mass.reduce((sum, value) => sum + value, 0);
  assert.ok(Math.abs(total - 10) < 1e-3, `mass ${total}`);
  assert.equal(at(density, 70, 40), density.peak, 'the peak sits on the bin');
  assert.ok(at(density, 67, 40) > 0 && Math.abs(at(density, 67, 40) - at(density, 73, 40)) < 1e-6, 'it falls off evenly');
  assert.ok(Math.abs(at(density, 70, 37) - at(density, 67, 40)) < 1e-6, 'and the same along both axes');
});

test('a wider kernel spreads the same marks lower and further', () => {
  const tight = heatDensity({ grid: GRID, bins: [[70, 40, 10]], bandwidth: heatBandwidth('de_nuke', SMOOTHING_MIN) });
  const wide = heatDensity({ grid: GRID, bins: [[70, 40, 10]], bandwidth: heatBandwidth('de_nuke', SMOOTHING_MAX) });
  assert.ok(tight && wide);
  assert.ok(wide.peak < tight.peak);
  assert.ok(at(wide, 75, 40) > at(tight, 75, 40));
});

test('the contours come in bands of the heat scale and the map shows through', () => {
  const density = heatDensity({ grid: GRID, bins: [[50, 50, 9], [90, 50, 1]], bandwidth: 2 });
  assert.ok(density);
  const picture = densityPicture(density);
  assert.ok(picture);
  const busy = pixel(picture, 50, 50);
  assert.deepEqual(busy?.slice(0, 3), heatRgb(1), 'the densest spot is the top of the scale');
  assert.ok(busy && busy[3] < 255 && busy[3] > 200, 'even the top band stays translucent');
  const quiet = pixel(picture, 90, 50);
  assert.ok(quiet && quiet[3] > 0 && quiet[3] < busy[3], 'the quiet spot shows in a lower band');
  assert.equal(pixel(picture, 70, 50)?.[3], 0, 'between them the radar stays clear');
  const colours = new Set<string>();
  for (let i = 0; i < picture.rgba.length; i += 4) if (picture.rgba[i + 3] > 0) colours.add(picture.rgba.slice(i, i + 3).join(','));
  assert.ok(colours.size <= DENSITY_THRESHOLDS, `${colours.size} colours, at most one per band`);
  assert.ok(picture.x <= 47 && picture.x + picture.width >= 93, 'the picture holds both spots');
});

test('a shared peak keeps both floors on one scale', () => {
  const upper = heatDensity({ grid: GRID, bins: [[60, 60, 8]], bandwidth: 2 });
  const lower = heatDensity({ grid: GRID, bins: [[60, 60, 2]], bandwidth: 2 });
  assert.ok(upper && lower);
  const own = densityPicture(lower);
  const shared = densityPicture(lower, Math.max(upper.peak, lower.peak));
  assert.ok(own && shared);
  const ownCentre = pixel(own, 60, 60);
  const sharedCentre = pixel(shared, 60, 60);
  assert.ok(ownCentre && sharedCentre && sharedCentre[3] < ownCentre[3], 'the quieter floor reads quieter on the shared scale');
});

test('timing colours by the arrival tone smoothed by weight, not by the count', () => {
  const density = heatDensity({ grid: GRID, bins: [[40, 40, 50], [41, 40, 1]], bandwidth: 1.5, tone: [1, 0] });
  assert.ok(density?.tone);
  assert.ok(density.tone[40 * GRID + 40] > 0.9, 'the early, crowded bin keeps its tone');
  const late = heatDensity({ grid: GRID, bins: [[40, 40, 50]], bandwidth: 1.5, tone: [0] });
  assert.ok(late);
  const picture = densityPicture(late);
  assert.deepEqual(pixel(picture ?? assert.fail('no picture'), 40, 40)?.slice(0, 3), heatRgb(0), 'a crowded but late spot is the cold end');
});

test('nothing to paint gives no density', () => {
  assert.equal(heatDensity({ grid: GRID, bins: [], bandwidth: 2 }), null);
  assert.equal(heatDensity({ grid: 0, bins: [[1, 1, 1]], bandwidth: 2 }), null);
  assert.equal(heatDensity({ grid: GRID, bins: [[-1, 4, 3], [150, 4, 3]], bandwidth: 2 }), null);
});

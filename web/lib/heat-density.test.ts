import test from 'node:test';
import assert from 'node:assert/strict';
import { DENSITY_FLOOR, SMOOTHING_MAX, SMOOTHING_MIN, densityPicture, heatBandwidth, heatDensity, heatTone } from './heat-density.ts';
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

test('the wash runs from blue to pink with the density and the map shows through', () => {
  const density = heatDensity({ grid: GRID, bins: [[50, 50, 9], [90, 50, 1]], bandwidth: 2 });
  assert.ok(density);
  const picture = densityPicture(density);
  assert.ok(picture);
  const busy = pixel(picture, 50, 50);
  const pink = heatRgb(1);
  assert.ok(busy && busy.slice(0, 3).every((channel, i) => Math.abs(channel - pink[i]) <= 6), 'the densest spot is pink');
  assert.ok(busy && busy[3] < 255 && busy[3] > 200, 'even the top stays translucent');
  const quiet = pixel(picture, 90, 50);
  assert.ok(quiet && quiet[3] > 0 && quiet[3] < busy[3], 'the quiet spot is fainter');
  assert.ok(quiet && quiet[2] > quiet[0] && quiet[0] < 160, 'and blue to purple, not pink');
  assert.equal(pixel(picture, 70, 50)?.[3], 0, 'between them the radar stays clear');
  assert.ok(picture.x <= 47 && picture.x + picture.width >= 93, 'the picture holds both spots');
});

test('the wash is continuous: no band edges, and the alpha falls off evenly towards the clear', () => {
  const density = heatDensity({ grid: GRID, bins: [[75, 75, 20]], bandwidth: 3 });
  assert.ok(density);
  const picture = densityPicture(density);
  assert.ok(picture);
  const colours = new Set<string>();
  const width = picture.width * picture.scale;
  const row = Math.floor((75.5 - picture.y) * picture.scale);
  const from = Math.floor((75.5 - picture.x) * picture.scale);
  let last = -1;
  let jump = 0;
  for (let px = from; px < width; px++) {
    const i = (row * width + px) * 4;
    colours.add(`${picture.rgba[i]},${picture.rgba[i + 1]},${picture.rgba[i + 2]}`);
    if (last >= 0) jump = Math.max(jump, Math.abs(last - picture.rgba[i + 3]));
    last = picture.rgba[i + 3];
  }
  assert.ok(colours.size >= 30, `${colours.size} colours along the slope, want a smooth run and not a few bands`);
  assert.ok(jump <= 24, `alpha steps by at most ${jump} between neighbouring pixels`);
});

test('the tone curve lifts the quiet end and keeps the ends', () => {
  assert.equal(heatTone(0), 0);
  assert.equal(heatTone(1), 1);
  assert.ok(heatTone(0.25) > 0.4 && heatTone(0.25) < 0.5, 'a quarter of the peak is already purple-blue');
  assert.equal(heatTone(3), 1);
  assert.ok(DENSITY_FLOOR > 0 && DENSITY_FLOOR < 0.1);
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
  assert.deepEqual(pixel(picture ?? assert.fail('no picture'), 40, 40)?.slice(0, 3), heatRgb(0), 'a crowded but late spot is the blue end');
});

test('nothing to paint gives no density', () => {
  assert.equal(heatDensity({ grid: GRID, bins: [], bandwidth: 2 }), null);
  assert.equal(heatDensity({ grid: 0, bins: [[1, 1, 1]], bandwidth: 2 }), null);
  assert.equal(heatDensity({ grid: GRID, bins: [[-1, 4, 3], [150, 4, 3]], bandwidth: 2 }), null);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import type { HeatmapCell } from './api/spatial.ts';
import type { RadarCalibration } from './api/tactical.ts';
import { densityRect, densityRgba, heatDensity, heatSigma } from './heat-density.ts';
import type { HeatDensity } from './heat-density.ts';
import { heatRgb } from './tactical-spatial.ts';

const MIRAGE: RadarCalibration = { map: 'de_mirage', source: 'overview', pos_x: -3230, pos_y: 1713, scale: 5, size: 1024 };
const NUKE: RadarCalibration = { map: 'de_nuke', source: 'overview', pos_x: -3453, pos_y: 2887, scale: 7, size: 1024 };

function cell(x: number, y: number, count: number, seconds = 0): HeatmapCell {
  return { side: 'T', level: 'default', x, y, count, rounds: 1, seconds, examples: [] };
}

/** The grid value at a world point. */
function at(grid: HeatDensity, values: Float32Array, x: number, y: number): number {
  const c = Math.floor((x - grid.minX) / grid.step);
  const r = Math.floor((grid.maxY - y) / grid.step);
  return values[r * grid.cols + c];
}

test('the kernel width follows the map and the smoothing, clamped', () => {
  assert.ok(Math.abs(heatSigma(MIRAGE, 1) - 75.09) < 0.01);
  assert.ok(heatSigma(NUKE, 1) < heatSigma({ ...NUKE, map: 'de_ancient' }, 1), 'Nuke is the tightest map');
  assert.equal(heatSigma(MIRAGE, 9), heatSigma(MIRAGE, 1.5));
  assert.equal(heatSigma(MIRAGE, 0), heatSigma(MIRAGE, 0.5));
  assert.equal(heatSigma(MIRAGE, Number.NaN), heatSigma(MIRAGE, 1));
});

test('a single cell spreads into a smooth bump that keeps its count', () => {
  const grid = heatDensity({ cells: [cell(2, -3, 10)], cellSize: 64, sigma: 75 });
  assert.ok(grid);
  const total = grid.mass.reduce((sum, value) => sum + value, 0);
  assert.ok(Math.abs(total - 10) < 0.05, `mass ${total}`);
  const centre = at(grid, grid.mass, 2.5 * 64, -2.5 * 64);
  assert.ok(Math.abs(centre - grid.peak) / grid.peak < 0.02, 'the peak sits on the cell');
  const left = at(grid, grid.mass, 2.5 * 64 - 100, -2.5 * 64);
  const right = at(grid, grid.mass, 2.5 * 64 + 100, -2.5 * 64);
  assert.ok(left > 0 && left < centre && Math.abs(left - right) / left < 0.05, 'it falls off evenly on both sides');
  const rect = densityRect(grid);
  assert.ok(rect.minX < 2 * 64 - 3 * 75 && rect.maxX > 3 * 64 + 3 * 75, 'the grid has room for the tail');
});

test('the busier spot is the denser one, and its pixels reach the top of the scale', () => {
  const grid = heatDensity({ cells: [cell(0, 0, 9), cell(20, 0, 1)], cellSize: 64, sigma: 60 });
  assert.ok(grid);
  const busy = at(grid, grid.mass, 32, 32);
  const quiet = at(grid, grid.mass, 20.5 * 64, 32);
  assert.ok(busy > 5 * quiet);
  const pixels = densityRgba(grid);
  const index = (Math.floor((grid.maxY - 32) / grid.step) * grid.cols + Math.floor((32 - grid.minX) / grid.step)) * 4;
  const [r, g, b] = heatRgb(Math.sqrt(busy / grid.peak));
  assert.deepEqual([pixels[index], pixels[index + 1], pixels[index + 2]], [r, g, b]);
  assert.ok(pixels[index + 3] > 180 && pixels[index + 3] < 255, 'the peak stays translucent');
  assert.equal(pixels[3], 0, 'the corner, far from every cell, is clear');
});

test('a shared peak keeps both floors on one scale', () => {
  const upper = heatDensity({ cells: [cell(0, 0, 8)], cellSize: 64, sigma: 60 });
  const lower = heatDensity({ cells: [cell(0, 0, 2)], cellSize: 64, sigma: 60 });
  assert.ok(upper && lower);
  const peak = Math.max(upper.peak, lower.peak);
  const own = densityRgba(lower);
  const shared = densityRgba(lower, peak);
  const centre = (Math.floor(lower.rows / 2) * lower.cols + Math.floor(lower.cols / 2)) * 4;
  assert.ok(shared[centre + 3] < own[centre + 3], 'the quieter floor reads quieter on the shared scale');
});

test('timing smooths each cell strength by its weight instead of the count', () => {
  const grid = heatDensity({ cells: [cell(0, 0, 50, 5), cell(1, 0, 1, 40)], cellSize: 64, sigma: 40, tone: (c) => (c.seconds < 10 ? 1 : 0) });
  assert.ok(grid?.tone);
  assert.ok(at(grid, grid.tone, 32, 32) > 0.9, 'the early, crowded cell keeps its strength');
  assert.equal(heatDensity({ cells: [], cellSize: 64, sigma: 40 }), null);
  assert.equal(heatDensity({ cells: [cell(0, 0, 1)], cellSize: 0, sigma: 40 }), null);
});

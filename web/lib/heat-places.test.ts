import test from 'node:test';
import assert from 'node:assert/strict';
import type { HeatmapCell } from './api/spatial.ts';
import { heatPlaces } from './heat-places.ts';

function cell(options: { x: number; y: number; count: number; level?: string; seconds?: number; rounds?: number; round?: number }): HeatmapCell {
  const { x, y, count, level = 'default', seconds = 10, rounds = 2, round = 1 } = options;
  return { side: 'CT', level, x, y, count, rounds, seconds, examples: [{ match_id: 'm', round, tick: 100 }] };
}

// The CT position cells of one real Nuke match: three of the hottest cells are CS2's Rafters.
const NAMES = new Map([
  ['13,-5', 'Rafters'],
  ['14,-5', 'Rafters'],
  ['10,-10', 'Rafters'],
  ['12,-25', 'Mini'],
  ['5,5', 'Ramp'],
  ['9,3', 'Ramp (piso inferior)'],
]);
const nameOf = (c: HeatmapCell): string => NAMES.get(`${c.x},${c.y}`) ?? '';

test('heat places: the cells of one named place are one line, with their total', () => {
  const places = heatPlaces({
    cells: [cell({ x: 12, y: -25, count: 756 }), cell({ x: 13, y: -5, count: 492, round: 1 }), cell({ x: 14, y: -5, count: 473, round: 2 }), cell({ x: 10, y: -10, count: 306, round: 3, rounds: 11 })],
    nameOf,
  });
  assert.deepEqual(
    places.map((place) => [place.name, place.count]),
    [['Rafters', 1271], ['Mini', 756]],
  );
  const rafters = places[0];
  assert.equal(rafters.cells.length, 3);
  assert.equal(rafters.rounds, 11);
  assert.deepEqual(rafters.examples.map((example) => example.round), [1, 2, 3]);
  assert.equal(new Set(places.map((place) => place.name)).size, places.length);
});

test('heat places: a name on two floors stays two places, told apart by their names', () => {
  const places = heatPlaces({ cells: [cell({ x: 5, y: 5, count: 40 }), cell({ x: 9, y: 3, count: 30, level: 'lower' })], nameOf });
  assert.deepEqual(places.map((place) => [place.name, place.level]), [['Ramp', 'default'], ['Ramp (piso inferior)', 'lower']]);
});

test('heat places: the time of a place is the median of its cells, weighted by what they counted', () => {
  const places = heatPlaces({ cells: [cell({ x: 13, y: -5, count: 10, seconds: 5 }), cell({ x: 14, y: -5, count: 60, seconds: 30 }), cell({ x: 10, y: -10, count: 30, seconds: 50 })], nameOf });
  assert.equal(places[0].seconds, 30);
});

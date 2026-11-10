import assert from 'node:assert/strict';
import test from 'node:test';
import { dayMonthLabel, slashDayLabel } from './day-label.ts';

test('the heading reads weekday, day, month and year without commas or dots', () => {
  assert.equal(slashDayLabel('2026-11-09'), 'lun 9 nov 2026');
  assert.equal(slashDayLabel('2026-11-06'), 'vie 6 nov 2026');
});

test('a day that does not parse is returned as it came', () => {
  assert.equal(slashDayLabel('sin fecha'), 'sin fecha');
  assert.equal(dayMonthLabel('sin fecha'), '');
});

test('the thumbnail day keeps two digits', () => {
  assert.equal(dayMonthLabel('2026-11-05T12:00:00'), '05 nov');
});

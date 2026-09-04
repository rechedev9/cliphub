import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, RETIRED_ROUTES } from './nav.ts';

/** Rail order: the number is the padded index, not a historical slot. */
const RAIL = [
  ['01', 'Clips y vídeos', '/clips'],
  ['02', 'Clips de stream', '/streams'],
  ['03', 'Jugadores', '/players'],
  ['04', 'Táctica', '/tactical'],
  ['05', 'Anti-cheat', '/cheaters'],
  ['06', 'Ajustes', '/settings'],
] as const;

test('nav: numbers follow rail order', () => {
  assert.deepEqual(
    NAV_SECTIONS.map((section) => [section.number, section.label, section.href]),
    RAIL,
  );
});

test('nav: numbers are unique and sequential from 01', () => {
  const numbers = NAV_SECTIONS.map((section) => section.number);
  assert.equal(new Set(numbers).size, numbers.length);
  for (const [i, number] of numbers.entries()) {
    assert.equal(number, String(i + 1).padStart(2, '0'));
  }
});

test('nav: hrefs are unique', () => {
  const hrefs = NAV_SECTIONS.map((section) => section.href);
  assert.equal(new Set(hrefs).size, hrefs.length);
});

test('nav: every retired door lands inside a live section', () => {
  const live = new Set<string>(NAV_SECTIONS.map((section) => section.href));
  for (const [from, to] of Object.entries(RETIRED_ROUTES)) {
    assert.equal(live.has(from), false, `${from} is retired and must not be in the rail`);
    const target = to.split('?')[0].split('/').slice(0, 2).join('/');
    assert.equal(live.has(target), true, `${from} → ${to} must land on a live section`);
  }
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, navSection } from './nav.ts';

/** Rail order: the number is the padded index, not a historical slot. */
const RAIL = [
  ['00', 'Inicio', '/onboarding'],
  ['01', 'Demos', '/matches'],
  ['02', 'Shorts', '/upload'],
  ['03', 'Clips de stream', '/streams'],
  ['04', 'Vídeos largos', '/full-demo'],
  ['05', 'Táctica', '/tactical'],
  ['06', 'CheaterDetect', '/cheaters'],
  ['07', 'Jugadores', '/players'],
  ['08', 'Biblioteca', '/videos'],
  ['09', 'Ajustes', '/settings'],
] as const;

test('nav: numbers follow rail order', () => {
  assert.deepEqual(
    NAV_SECTIONS.map((section) => [section.number, section.label, section.href]),
    RAIL,
  );
});

test('nav: numbers are unique and sequential from 00', () => {
  const numbers = NAV_SECTIONS.map((section) => section.number);
  assert.equal(new Set(numbers).size, numbers.length);
  for (const [i, number] of numbers.entries()) {
    assert.equal(number, String(i).padStart(2, '0'));
  }
});

test('nav: hrefs are unique', () => {
  const hrefs = NAV_SECTIONS.map((section) => section.href);
  assert.equal(new Set(hrefs).size, hrefs.length);
});

for (const [number, label, href] of RAIL) {
  test(`navSection: ${href} is ${number} ${label}`, () => {
    const entry = navSection(href);
    assert.equal(entry.number, number);
    assert.equal(entry.label, label);
  });
}

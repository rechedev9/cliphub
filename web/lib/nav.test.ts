import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, RETIRED_ROUTES } from './nav.ts';

const RAIL = [
  ['Clips y vídeos', '/clips'],
  ['Clips de stream', '/streams'],
  ['Jugadores', '/players'],
  ['Táctica', '/tactical'],
  ['Anti-cheat', '/cheaters'],
  ['Ajustes', '/settings'],
] as const;

test('nav: sections follow rail order', () => {
  assert.deepEqual(
    NAV_SECTIONS.map((section) => [section.label, section.href]),
    RAIL,
  );
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

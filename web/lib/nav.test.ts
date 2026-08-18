import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, navSection } from './nav.ts';

test('nav: the entry section plus eleven numbered destinations', () => {
  assert.equal(NAV_SECTIONS.length, 12);
});

test('nav: Inicio is 00 so the eleven destinations keep their numbers', () => {
  // The numbers are spoken in the command strip and in the design handoff's own
  // piece codes, so seating Inicio must not renumber anything below it.
  const [entry, ...destinations] = NAV_SECTIONS;
  assert.equal(entry.number, '00');
  assert.equal(entry.href, '/onboarding');
  destinations.forEach((section, index) => {
    assert.equal(section.number, String(index + 1).padStart(2, '0'));
  });
});

test('nav: hrefs are unique', () => {
  const hrefs = NAV_SECTIONS.map((section) => section.href);
  assert.equal(new Set(hrefs).size, hrefs.length);
});

test('navSection: returns the entry for a known href', () => {
  const entry = navSection('/videos');
  assert.equal(entry.number, '09');
  assert.equal(entry.label, 'Biblioteca');
});

test('navSection: tactical sits with the demo-derived sections', () => {
  const entry = navSection('/tactical');
  assert.equal(entry.number, '03');
  assert.equal(entry.label, 'Táctica');
});

test('navSection: CheaterDetect sits with the demo-analysis sections', () => {
  const entry = navSection('/cheaters');
  assert.equal(entry.number, '04');
  assert.equal(entry.label, 'CheaterDetect');
});

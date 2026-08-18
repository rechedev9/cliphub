import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, navSection } from './nav.ts';

test('nav: the entry section plus twelve numbered destinations', () => {
  assert.equal(NAV_SECTIONS.length, 13);
});

test('nav: Inicio is 00 and 01-11 stay put when Full demo is added', () => {
  const byHref = Object.fromEntries(NAV_SECTIONS.map((section) => [section.href, section.number]));
  assert.equal(byHref['/onboarding'], '00');
  assert.equal(byHref['/matches'], '01');
  assert.equal(byHref['/upload'], '02');
  assert.equal(byHref['/full-demo'], '12');
  assert.equal(byHref['/tactical'], '03');
  assert.equal(byHref['/settings'], '11');
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

test('navSection: Full demo to video sits with the demo production sections', () => {
  const entry = navSection('/full-demo');
  assert.equal(entry.number, '12');
  assert.equal(entry.label, 'Full demo to video');
});

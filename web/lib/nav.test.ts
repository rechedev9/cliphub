// Unit tests for the shared Studio nav data: the sidebar and every page
// header derive their numbering from NAV_SECTIONS, so the invariants below
// (zero-padded, sequential, unique hrefs) keep them from drifting again.
// The group separator is data too, so it is pinned here rather than in a
// hardcoded index inside the sidebar component.
// Run: node --test "lib/**/*.test.ts"
import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_SECTIONS, navSection } from './nav.ts';

test('nav: exactly 8 sections', () => {
  assert.equal(NAV_SECTIONS.length, 8);
});

test('nav: numbers are zero-padded and sequential from 01', () => {
  NAV_SECTIONS.forEach((section, index) => {
    assert.equal(section.number, String(index + 1).padStart(2, '0'));
  });
});

test('nav: hrefs are unique', () => {
  const hrefs = NAV_SECTIONS.map((section) => section.href);
  assert.equal(new Set(hrefs).size, hrefs.length);
});

test('navSection: returns the entry for a known href', () => {
  const entry = navSection('/videos');
  assert.equal(entry.number, '06');
  assert.equal(entry.label, 'Biblioteca');
});

test('navSection: tactical sits with the demo-derived sections', () => {
  const entry = navSection('/tactical');
  assert.equal(entry.number, '03');
  assert.equal(entry.label, 'Táctica');
});

test('nav: exactly one entry opens the content group', () => {
  const starts = NAV_SECTIONS.filter((section) => section.startsGroup);
  assert.equal(starts.length, 1);
  assert.equal(starts[0].href, '/videos');
});

test('nav: the first entry never draws a separator above it', () => {
  assert.equal(NAV_SECTIONS[0].startsGroup, false);
});

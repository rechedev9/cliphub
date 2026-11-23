import test from 'node:test';
import assert from 'node:assert/strict';
import { TOUR_CHAPTERS } from './app-tour.ts';
import { NAV_SECTIONS, RETIRED_ROUTES, sectionForPath } from './nav.ts';
import { SIDEBAR_FOOTER_ENTRIES, SIDEBAR_GROUPS } from './nav-groups.ts';

function linkPath(href: string): string {
  return href.split(/[?#]/)[0] ?? href;
}

function sectionOf(href: string): (typeof NAV_SECTIONS)[number] | undefined {
  return sectionForPath(linkPath(href)) ?? undefined;
}

test('app tour: every rail section is explained by a chapter wearing its label', () => {
  for (const section of NAV_SECTIONS) {
    const chapter = TOUR_CHAPTERS.find(
      (entry) => entry.link !== undefined && sectionOf(entry.link.href)?.href === section.href && entry.kicker === section.label,
    );
    assert.ok(chapter, `no tour chapter links to ${section.href} with kicker ${section.label}`);
  }
});

test('app tour: a chapter that borrows a rail label links inside that section', () => {
  for (const chapter of TOUR_CHAPTERS) {
    if (chapter.link === undefined) continue;
    const section = sectionOf(chapter.link.href);
    assert.ok(section, `${chapter.id} links outside the rail: ${chapter.link.href}`);
    const railed = NAV_SECTIONS.some((entry) => entry.label === chapter.kicker);
    if (railed && chapter.id !== 'capture') assert.equal(chapter.kicker, section.label, chapter.id);
  }
});

test('app tour: no chapter links to a retired route', () => {
  const retired = new Set<string>(Object.keys(RETIRED_ROUTES));
  for (const chapter of TOUR_CHAPTERS) {
    if (chapter.link === undefined) continue;
    assert.equal(retired.has(linkPath(chapter.link.href)), false, `${chapter.id} → ${chapter.link.href}`);
  }
});

test('app tour: ids and point terms are unique so React keys stay stable', () => {
  const ids = TOUR_CHAPTERS.map((chapter) => chapter.id);
  assert.equal(new Set(ids).size, ids.length);
  for (const chapter of TOUR_CHAPTERS) {
    const terms = chapter.points.map((point) => point.term);
    assert.equal(new Set(terms).size, terms.length, chapter.id);
    assert.ok(chapter.points.length > 0, `${chapter.id} has no points`);
  }
});

test('app tour: the chapters walk the rail in its order and the welcome names every group', () => {
  const rail = [...SIDEBAR_GROUPS.flatMap((group) => group.entries), ...SIDEBAR_FOOTER_ENTRIES].map((entry) => entry.label);
  const railed = new Set(rail);
  assert.deepEqual(TOUR_CHAPTERS.map((chapter) => chapter.kicker).filter((kicker) => railed.has(kicker)), rail);
  const welcome = TOUR_CHAPTERS[0]?.points.map((point) => point.text).join(' ') ?? '';
  for (const group of SIDEBAR_GROUPS) assert.ok(welcome.includes(group.label), group.label);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import { NAV_AREA_LABEL, NAV_SECTIONS, RETIRED_ROUTES, retiredHref, sectionForPath } from './nav.ts';

const RAIL = [
  ['create', 'Demo completa', '/videos-largos'],
  ['create', 'Short', '/shorts'],
  ['create', 'Clips de stream', '/streams'],
  ['create', 'Mis vídeos', '/mis-videos'],
  ['create', 'Ideas', '/ideas'],
  ['analyze', 'Partidas', '/partidas'],
  ['analyze', 'Equipos', '/equipos'],
  ['analyze', 'Jugadores', '/jugadores'],
  ['analyze', 'Stratbook', '/stratbook'],
  ['analyze', 'Informes', '/informes'],
  ['analyze', 'Integridad', '/integridad'],
  ['chrome', 'Ajustes', '/settings'],
] as const;

test('nav: sections follow rail order, split into Crear and Analizar', () => {
  assert.deepEqual(
    NAV_SECTIONS.map((section) => [section.area, section.label, section.href]),
    RAIL,
  );
  assert.deepEqual(NAV_AREA_LABEL, { create: 'Crear', analyze: 'Analizar' });
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

test('nav: a nested route lights its section, a series lights Short', () => {
  assert.equal(sectionForPath('/partidas/1b9d6bcd-bbfd-4b2d-9b5d-ab8dfbbd4bed')?.href, '/partidas');
  assert.equal(sectionForPath('/mis-videos/abc/publicar/clip')?.href, '/mis-videos');
  assert.equal(sectionForPath('/series/abc')?.href, '/shorts');
  assert.equal(sectionForPath('/shortsx'), null);
  assert.equal(sectionForPath('/'), null);
});

test('nav: a renamed section keeps the nested path and the query of the old URL', () => {
  assert.equal(retiredHref('/resultados'), '/mis-videos');
  assert.equal(
    retiredHref('/resultados', { rest: ['job 1', 'publicar', 'clip'], search: { origen: 'short', tag: ['a', 'b'], none: undefined } }),
    '/mis-videos/job%201/publicar/clip?origen=short&tag=a&tag=b',
  );
  assert.equal(retiredHref('/tactical', { rest: ['abc'], search: { round: '4' } }), '/partidas/abc?round=4');
  // Segments may arrive still encoded, or malformed; neither is encoded twice nor thrown on.
  assert.equal(retiredHref('/resultados', { rest: ['job%201', '100%'] }), '/mis-videos/job%201/100%25');
});

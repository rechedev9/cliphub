import test from 'node:test';
import assert from 'node:assert/strict';
import { RESULTS_HREF } from './clips/routes.ts';
import { currentEdition, homePathFor, isPathAvailable, navSectionsFor } from './edition.ts';
import { NAV_SECTIONS } from './nav.ts';

function withEdition(value: string | undefined, run: () => void): void {
  const previous = process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
  if (value === undefined) delete process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
  else process.env.NEXT_PUBLIC_CLIPHUB_EDITION = value;
  try {
    run();
  } finally {
    if (previous === undefined) delete process.env.NEXT_PUBLIC_CLIPHUB_EDITION;
    else process.env.NEXT_PUBLIC_CLIPHUB_EDITION = previous;
  }
}

test('edition: desktop is the default and keeps its home', () => {
  withEdition(undefined, () => {
    assert.equal(currentEdition(), 'desktop');
    assert.equal(homePathFor('desktop'), RESULTS_HREF);
  });
  withEdition('desktop', () => assert.equal(currentEdition(), 'desktop'));
  withEdition('WEB', () => assert.equal(currentEdition(), 'desktop'));
});

test('edition: only the exact web value selects the web edition', () => {
  withEdition('web', () => {
    assert.equal(currentEdition(), 'web');
    assert.equal(homePathFor('web'), '/partidas');
  });
});

test('edition: desktop pages hide their nested paths but not lookalike prefixes', () => {
  assert.equal(isPathAvailable('/shorts', 'web'), false);
  assert.equal(isPathAvailable('/shorts/abc', 'web'), false);
  assert.equal(isPathAvailable('/videos-largos/serie/abc', 'web'), false);
  assert.equal(isPathAvailable('/series/abc', 'web'), false);
  assert.equal(isPathAvailable('/shortsx', 'web'), true);
  assert.equal(isPathAvailable('/descargar', 'web'), true);
  assert.equal(isPathAvailable('/shorts', 'desktop'), true);
});

test('edition: player analysis stays while Jugadores FACEIT is desktop only', () => {
  assert.equal(isPathAvailable('/jugadores', 'web'), true);
  assert.equal(isPathAvailable('/jugadores/76561198000000000', 'web'), true);
  assert.equal(isPathAvailable('/jugadores/faceit', 'web'), false);
  assert.equal(isPathAvailable('/jugadores/faceit/abc', 'web'), false);
});

test('edition: the desktop API prefixes hide with their nested paths', () => {
  const prefixes = [
    '/api/editor',
    '/api/full-demo',
    '/api/streams',
    '/api/imported-videos',
    '/api/songs',
    '/api/presets',
    '/api/practice',
    '/api/voice-plugin',
    '/api/steam',
    '/api/demos',
  ];
  for (const prefix of prefixes) {
    assert.equal(isPathAvailable(prefix, 'web'), false, prefix);
    assert.equal(isPathAvailable(`${prefix}/nested`, 'web'), false, prefix);
    assert.equal(isPathAvailable(prefix, 'desktop'), true, prefix);
  }
  assert.equal(isPathAvailable('/api/demosx', 'web'), true);
  assert.equal(isPathAvailable('/api/library/demos', 'web'), true);
});

test('edition: retired doors follow their desktop target', () => {
  assert.equal(isPathAvailable('/resultados', 'web'), false);
  assert.equal(isPathAvailable('/players', 'web'), false);
  assert.equal(isPathAvailable('/tactical', 'web'), true);
  assert.equal(isPathAvailable('/stratbook', 'web'), true);
});

test('edition: the web rail has no create area and desktop keeps every section', () => {
  const web = navSectionsFor('web');
  assert.equal(web.some((section) => section.area === 'create'), false);
  assert.equal(web.some((section) => section.href === '/almacenamiento'), false);
  assert.deepEqual(navSectionsFor('desktop'), NAV_SECTIONS);
});

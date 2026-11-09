import test from 'node:test';
import assert from 'node:assert/strict';
import type { ProLineup } from './api/pro-lineups.ts';
import {
  DEFAULT_QUERY,
  explorerLineups,
  landingSpots,
  plainTitle,
  readExplorerQuery,
  spotHref,
  spotOf,
  visibleSpots,
  writeExplorerQuery,
} from './pro-lineups-view.ts';

type Shape = { id: string; kind?: string; side?: string; place?: string; at: [number, number]; throws: number; clip?: string };

const lineup = ({ id, kind = 'smoke', side = 'T', place = 'Window', at, throws, clip }: Shape) =>
  ({
    id,
    kind,
    side,
    insta: false,
    title: id,
    land_place: place,
    throw_place: 'T Spawn',
    landing: [at[0], at[1], 0],
    throws: { recent: throws, all: throws },
    ...(clip === undefined ? {} : { clip }),
  }) as unknown as ProLineup;

test('lineups of one grenade landing close together at one callout are one spot, most thrown first', () => {
  const spots = landingSpots([
    lineup({ id: 'a', at: [0, 0], throws: 5 }),
    lineup({ id: 'b', at: [100, 0], throws: 40 }),
    lineup({ id: 'c', at: [900, 0], throws: 30 }),
    lineup({ id: 'd', kind: 'flash', at: [0, 0], throws: 50 }),
    lineup({ id: 'e', place: 'Jungle', at: [50, 0], throws: 1 }),
  ]);
  assert.deepEqual(
    spots.map((s) => [s.id, s.lineups.map((l) => l.id), s.throws]),
    [
      ['d', ['d'], 50],
      ['b', ['b', 'a'], 45],
      ['c', ['c'], 30],
      ['e', ['e'], 1],
    ],
  );
  assert.equal(spotOf(spots, 'a')?.id, 'b');
});

test('filters keep a spot where it is and drop it once it has no lineups left', () => {
  const spots = landingSpots([lineup({ id: 'a', at: [0, 0], throws: 5 }), lineup({ id: 'b', at: [100, 0], throws: 40, side: 'CT' })]);
  const tOnly = visibleSpots(spots, new Set(['a']));
  assert.deepEqual(tOnly.map((s) => [s.id, s.landing, s.throws]), [['b', [100, 0, 0], 5]]);
  assert.deepEqual(visibleSpots(spots, new Set()), []);
});

test('the explorer state round-trips through the URL and defaults stay out of it', () => {
  assert.equal(writeExplorerQuery(DEFAULT_QUERY), '');
  const state = { ...DEFAULT_QUERY, view: 'combos' as const, kind: 'all' as const, side: 'CT' as const, video: true, text: 'B site', spot: 'x', lineup: 'y', combo: 'z', level: 'lower' as const };
  assert.deepEqual(readExplorerQuery(writeExplorerQuery(state)), state);
  assert.deepEqual(readExplorerQuery('granada=decoy&lado=X&vista=nope'), DEFAULT_QUERY);
});

test('the explorer opens on smokes and the video filter is off', () => {
  const lineups = [lineup({ id: 'a', at: [0, 0], throws: 1 }), lineup({ id: 'b', kind: 'flash', at: [0, 0], throws: 1, clip: '1/b.mp4' })];
  assert.deepEqual(explorerLineups(lineups, DEFAULT_QUERY).map((l) => l.id), ['a']);
  assert.deepEqual(explorerLineups(lineups, { ...DEFAULT_QUERY, kind: 'all', video: true }).map((l) => l.id), ['b']);
});

test('a variant number leaves the title and the way back opens the lineup inside its spot', () => {
  assert.equal(plainTitle('Humo a Window desde T Spawn (jumpthrow) · variante 7'), 'Humo a Window desde T Spawn (jumpthrow)');
  assert.equal(plainTitle('Humo a Window'), 'Humo a Window');
  assert.equal(spotHref('de_mirage', { id: 'smoke-t-1', kind: 'flash' }), '/lineups/de_mirage?granada=flash&destino=smoke-t-1&lineup=smoke-t-1');
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  areaOfBox,
  areaOfPolygon,
  areaShape,
  blockSentence,
  boxOfArea,
  blockTramo,
  entryLabel,
  inArea,
  patternFromQuery,
  patternReplayRefs,
  timedKind,
  trackInWindow,
  withTramo,
  writePatternQuery,
} from './pattern-finder.ts';
import type { PatternDefinition, PatternResult } from '../api/patterns.ts';

test('a pattern survives the URL and a broken one reads as none', () => {
  const pattern: PatternDefinition = {
    side: 'CT',
    blocks: [
      { kind: 'player', players: ['76561198000000001'], area: { min_x: -10, min_y: 0, max_x: 300, max_y: 200, level: 'lower' }, from: 0, to: 15, invert: true },
      { kind: 'utility', rival: true, grenade: 'smoke', from_throw: true, from: 5 },
    ],
  };
  const query = new URLSearchParams(writePatternQuery('tab=replay&forma=patrones', pattern));
  assert.equal(query.get('tab'), 'replay');
  assert.deepEqual(patternFromQuery(query), pattern);
  assert.equal(writePatternQuery(query.toString(), null), 'tab=replay&forma=patrones');
  for (const bad of ['{', '{"blocks":[]}', '{"blocks":[{"kind":"kill","from":0}]}', '{"blocks":[{"kind":"player","from":20,"to":10}]}']) {
    assert.equal(patternFromQuery(new URLSearchParams({ patron: bad })), null, bad);
  }
  const loose = patternFromQuery(new URLSearchParams({ patron: '{"blocks":[{"kind":"player","from":0,"grenade":"smoke","players":["nick"]}]}' }));
  assert.deepEqual(loose, { blocks: [{ kind: 'player', from: 0 }] });
});

test('the block window maps to the tramo and back on the round clock', () => {
  assert.deepEqual(blockTramo({ kind: 'player', from: 0 }), { from: 0, to: 0 });
  assert.deepEqual(blockTramo({ kind: 'player', from: 0, to: 15 }), { from: 0, to: 15 });
  assert.deepEqual(withTramo({ kind: 'player', from: 3, to: 9 }, { from: 0, to: 0 }), { kind: 'player', from: 0 });
  assert.deepEqual(withTramo({ kind: 'player', from: 0 }, { from: 10, to: 115 }), { kind: 'player', from: 10 });
  assert.deepEqual(withTramo({ kind: 'player', from: 0 }, { from: 0, to: 0, fromPlant: true }), { kind: 'player', from: 0 });
  assert.equal(blockSentence({ kind: 'player', from: 0, to: 35, area: { min_x: 0, min_y: 0, max_x: 1, max_y: 1 } }), 'Pasa por la caja dibujada entre 1:55 y 1:20');
  assert.equal(blockSentence({ kind: 'utility', from_throw: true, from: 0, area: { min_x: 0, min_y: 0, max_x: 1, max_y: 1 } }), 'La tira desde la caja dibujada en toda la ronda');
  assert.equal(entryLabel({ kind: 'player', seconds: 13 }), 'Entra en la zona a 1:42');
  assert.equal(entryLabel({ kind: 'player', seconds: 52, plant: 40 }), 'Entra en la zona a +12s tras el plant');
  assert.equal(entryLabel({ kind: 'utility', seconds: 120 }), 'La tira a +5s tras el plant');
  assert.equal(entryLabel({ kind: 'plant', seconds: 60, plant: 60 }), 'Planta a +0s tras el plant');
  assert.equal(entryLabel({ kind: 'plant', seconds: 60 }), 'Planta a 0:55');
  // A player block without a zone has no moment of entry; the next timed block gives the row's time.
  assert.equal(timedKind({ blocks: [{ kind: 'player', from: 0 }, { kind: 'plant', from: 0 }] }), 'plant');
});

test('an area keeps the floor it was drawn on only on a two-floor map', () => {
  const box = { minX: -10.4, minY: 0, maxX: 99.6, maxY: 50 };
  assert.deepEqual(areaOfBox({ box, level: 'lower', twoFloors: false }), { min_x: -10, min_y: 0, max_x: 100, max_y: 50 });
  const lower = areaOfBox({ box, level: 'lower', twoFloors: true });
  assert.equal(lower.level, 'lower');
  assert.equal(inArea(lower, { x: 20, y: 20, level: 'lower' }), true);
  assert.equal(inArea(lower, { x: 20, y: 20, level: 'default' }), false);
  assert.equal(inArea(lower, { x: 200, y: 20, level: 'lower' }), false);
  assert.equal(inArea(areaOfBox({ box, level: 'default', twoFloors: false }), { x: 20, y: 20, level: 'lower' }), true);
});

test('a polygon area holds its inside, a callout area its callouts, and a count reads in the sentence', () => {
  // An L: the notch at the top right is outside.
  const l = areaOfPolygon({ points: [[0, 0], [100.4, 0], [100, 50], [50, 50], [50, 100], [0, 100]], level: 'default', twoFloors: false });
  assert.deepEqual(l.polygon?.[1], [100, 0]);
  assert.equal(areaShape(l), 'polygon');
  assert.equal(boxOfArea(l), null);
  assert.equal(inArea(l, { x: 25, y: 75, level: 'default' }), true);
  assert.equal(inArea(l, { x: 75, y: 75, level: 'default' }), false);
  const site = { callouts: ['BombsiteA', 'Ramp'] };
  assert.equal(inArea(site, { x: 0, y: 0, level: 'default', callout: 'Ramp' }), true);
  assert.equal(inArea(site, { x: 0, y: 0, level: 'default' }), false);
  const names = (token: string): string => (token === 'BombsiteA' ? 'Sitio A' : token);
  assert.equal(blockSentence({ kind: 'player', min_players: 3, area: site, from: 0 }, names), '3 o más a la vez en Sitio A o Ramp en toda la ronda');
  assert.equal(blockSentence({ kind: 'plant', area: l, from: 0 }), 'Planta en la zona dibujada en toda la ronda');

  const pattern: PatternDefinition = { blocks: [{ kind: 'player', min_players: 3, area: site, from: 0 }, { kind: 'player', area: { polygon: [[0, 0], [10, 0], [0, 10]], level: 'lower' }, from: 0 }] };
  assert.deepEqual(patternFromQuery(new URLSearchParams(writePatternQuery('', pattern))), pattern);
  for (const bad of [
    '{"blocks":[{"kind":"player","from":0,"area":{"polygon":[[0,0],[1,1]]}}]}',
    '{"blocks":[{"kind":"player","from":0,"area":{"callouts":["A"],"level":"lower"}}]}',
    '{"blocks":[{"kind":"player","from":0,"area":{"callouts":["A"],"polygon":[[0,0],[1,0],[0,1]]}}]}',
  ]) {
    assert.equal(patternFromQuery(new URLSearchParams({ patron: bad })), null, bad);
  }
});

test('a track keeps the points inside the clock window', () => {
  const track = { start: 2, points: [[0, 0, 0], [1, 0, 0], [2, 0, 0], [3, 0, 0], [4, 0, 0]] satisfies [number, number, number][] };
  // Points sit at 1, 1.5, 2, 2.5 and 3 seconds with a 0.5 s step.
  assert.deepEqual(trackInWindow({ track, step: 0.5, tramo: { from: 2, to: 10 } }).map((p) => p[0]), [2, 3, 4]);
  assert.equal(trackInWindow({ track, step: 0.5, tramo: { from: 0, to: 0 } }).length, 5);
  assert.equal(trackInWindow({ track, step: 0, tramo: { from: 0, to: 0 } }).length, 0);
});

test('the matched rounds open at the moment the pattern held', () => {
  const result: PatternResult = {
    rounds: 3, matched: 2, won: 1, win_rate: 0.5, blocks: [], missing_positions: 0, recent_rounds: 3, recent_matched: 2, trend: '', months: [],
    matches: [{ match_id: 'm1', played_at: '', rounds: 3, matched: [{ round: 4, side: 'T', won: true, hits: [], tick: 900 }, { round: 7, side: 'T', won: false, hits: [] }] }],
  };
  assert.deepEqual(patternReplayRefs(result), [{ matchId: 'm1', round: 4, tick: 900 }, { matchId: 'm1', round: 7 }]);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import {
  areaOfBox,
  blockSentence,
  blockTramo,
  entryLabel,
  inArea,
  patternFromQuery,
  patternReplayRefs,
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
  assert.equal(entryLabel('player', 13), 'Entra en la zona a 1:42');
  assert.equal(entryLabel('plant', 60), 'Planta a 0:55');
});

test('an area keeps the floor it was drawn on only on a two-floor map', () => {
  const box = { minX: -10.4, minY: 0, maxX: 99.6, maxY: 50 };
  assert.deepEqual(areaOfBox(box, 'lower', false), { min_x: -10, min_y: 0, max_x: 100, max_y: 50 });
  const lower = areaOfBox(box, 'lower', true);
  assert.equal(lower.level, 'lower');
  assert.equal(inArea(lower, 20, 20, 'lower'), true);
  assert.equal(inArea(lower, 20, 20, 'default'), false);
  assert.equal(inArea(lower, 200, 20, 'lower'), false);
  assert.equal(inArea(areaOfBox(box, 'default', false), 20, 20, 'lower'), true);
});

test('a track keeps the points inside the clock window', () => {
  const track = { start: 2, points: [[0, 0, 0], [1, 0, 0], [2, 0, 0], [3, 0, 0], [4, 0, 0]] satisfies [number, number, number][] };
  // Points sit at 1, 1.5, 2, 2.5 and 3 seconds with a 0.5 s step.
  assert.deepEqual(trackInWindow(track, 0.5, { from: 2, to: 10 }).map((p) => p[0]), [2, 3, 4]);
  assert.equal(trackInWindow(track, 0.5, { from: 0, to: 0 }).length, 5);
  assert.equal(trackInWindow(track, 0, { from: 0, to: 0 }).length, 0);
});

test('the matched rounds open at the moment the pattern held', () => {
  const result: PatternResult = {
    rounds: 3, matched: 2, won: 1, win_rate: 0.5, blocks: [], missing_positions: 0, recent_rounds: 3, recent_matched: 2, trend: '', months: [],
    matches: [{ match_id: 'm1', played_at: '', rounds: 3, matched: [{ round: 4, side: 'T', won: true, hits: [], tick: 900 }, { round: 7, side: 'T', won: false, hits: [] }] }],
  };
  assert.deepEqual(patternReplayRefs(result), [{ matchId: 'm1', round: 4, tick: 900 }, { matchId: 'm1', round: 7 }]);
});

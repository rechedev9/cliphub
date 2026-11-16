import { test } from 'node:test';
import assert from 'node:assert/strict';
import { autoPovNotes, autoPovSummary, autoPovUrl, cueLabel, parseAutoPov } from './auto-pov.ts';
import { victimReplaySeconds } from './short-selection.ts';

const JOB = '29b58ffc-4722-4c77-86c2-d5febb3f9960';

const ANSWER = {
  target_seconds: 180,
  total_seconds: 41,
  segment_ids: ['seg-002', '../x', 'seg-007', 9],
  plays: [
    { segment_id: 'seg-002', cues: [{ kind: 'clutch', detail: 'won', count: 3 }, { kind: 'momentum', detail: 'comeback', count: 4 }, { kind: 'zoom' }] },
    { segment_id: 'seg-007', cues: [{ kind: 'bomb', detail: 'retake', value: 0.9 }, { kind: 'duel', detail: 'later' }] },
  ],
  players: [
    { steamid64: '1', name: 'ropz', plays: 1 },
    { steamid64: '2', name: 'b1t', plays: 1 },
    { steamid64: '3', name: 'jL', plays: 0 },
    { name: 'no id' },
  ],
};

test('the Auto-POV answer keeps well-formed ids, known cues and players', () => {
  const got = parseAutoPov(ANSWER);
  assert.ok(got);
  assert.deepEqual(got.segmentIds, ['seg-002', 'seg-007']);
  assert.deepEqual(got.cues.get('seg-002')?.map((cue) => cue.kind), ['clutch', 'momentum']);
  assert.deepEqual(got.players.map((p) => p.name), ['ropz', 'b1t', 'jL']);
  assert.equal(parseAutoPov({ segment_ids: [] }), null);
  assert.equal(parseAutoPov([]), null);
});

test('cue labels name the observer reason in Spanish and drop unknown details', () => {
  const got = parseAutoPov(ANSWER);
  assert.ok(got);
  assert.deepEqual([...autoPovNotes(got)], [
    ['seg-002', 'Clutch 1v3 ganado · Corta 4 rondas perdidas'],
    ['seg-007', 'Retake con la bomba plantada'],
  ]);
  assert.equal(cueLabel({ kind: 'clutch', detail: '', count: 2, value: 0 }), 'Clutch 1v2');
  assert.equal(cueLabel({ kind: 'duel', detail: 'opening', count: 0, value: 0 }), 'Duelo de apertura');
  assert.equal(cueLabel({ kind: 'momentum', detail: 'run', count: 5, value: 0 }), 'Racha de 5 rondas');
  assert.equal(cueLabel({ kind: 'momentum', detail: 'swing', count: 0, value: 0.4 }), 'Gira la ronda');
});

test('the summary names only the players the pick left out', () => {
  const got = parseAutoPov(ANSWER);
  assert.ok(got);
  assert.equal(autoPovSummary(got), 'La elección automática no incluye ninguna jugada de jL.');
  assert.equal(autoPovSummary({ ...got, players: got.players.filter((p) => p.plays > 0) }), null);
});

test('the request carries the budget and the victim replay each play adds, capped', () => {
  const extra = victimReplaySeconds({ victimReplay: true, victimReplaySpeed: 0.25 });
  assert.equal(extra, 14);
  assert.equal(victimReplaySeconds({ victimReplay: false, victimReplaySpeed: 0.25 }), 0);
  assert.equal(autoPovUrl(JOB, { targetSeconds: 180, extraSeconds: extra }), `/api/demos/${JOB}/auto-pov?target_seconds=180&extra_seconds=14`);
  assert.equal(autoPovUrl(JOB, { targetSeconds: 60, extraSeconds: 40 }), `/api/demos/${JOB}/auto-pov?target_seconds=60&extra_seconds=15`);
});

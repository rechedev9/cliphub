import test from 'node:test';
import assert from 'node:assert/strict';
import { planToPlays, type KillPlan } from '../api/map.ts';
import { preRolledSelection } from './pre-roll.ts';
import { estimatedSelectionSeconds } from './short-selection.ts';

const TICKRATE = 64;
const at = (seconds: number): number => seconds * TICKRATE;

// One player: two plays of round 2 nine seconds apart, then one play of round 3.
const plan: KillPlan = {
  demo: { tickrate: TICKRATE },
  segments: [
    { id: 'seg-001', round: 2, tick_start: at(97), tick_end: at(110), kills: [{ tick: at(100) }, { tick: at(105) }] },
    { id: 'seg-002', round: 2, tick_start: at(111), tick_end: at(119), kills: [{ tick: at(114) }] },
    { id: 'seg-003', round: 3, tick_start: at(197), tick_end: at(205), kills: [{ tick: at(200) }] },
  ],
};
const plays = planToPlays('job', plan);
const all = new Set(plays.map((play) => play.id));

function estimate(preRollSeconds: 3 | 5 | 8, selectedIds: ReadonlySet<string> = all): number {
  return estimatedSelectionSeconds(preRolledSelection(plays, { selectedIds, preRollSeconds }));
}

test('the estimated length grows with the seconds before each kill', () => {
  assert.equal(estimate(3), 29, '13 + 8 + 8 s, the analysis cut');
  assert.equal(estimate(5), 34, 'round 2 becomes one play of 24 s, round 3 lasts 10 s');
  assert.equal(estimate(8), 40, 'round 2 becomes one play of 27 s, round 3 lasts 13 s');
});

test('plays the longer lead-in joins are recorded once, under the earlier play', () => {
  assert.deepEqual(preRolledSelection(plays, { selectedIds: all, preRollSeconds: 5 }).map((play) => play.id), ['seg-001', 'seg-003']);
  assert.deepEqual(preRolledSelection(plays, { selectedIds: all, preRollSeconds: 3 }).map((play) => play.id), ['seg-001', 'seg-002', 'seg-003']);
  assert.equal(estimate(5, new Set(['seg-002'])), 24, 'the later play alone still records the joined window');
  assert.equal(estimate(3, new Set(['seg-002'])), 8);
});

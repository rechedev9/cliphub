import assert from 'node:assert/strict';
import test from 'node:test';
import { planToPlays } from './map.ts';

test('planToPlays removes duplicate timeline segments but preserves distinct plays', () => {
  const plays = planToPlays('job-1', {
    segments: [
      { id: 'first', round: 4, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] },
      { id: 'duplicate', round: 4, tick_start: 100, tick_end: 200, kills: [{ weapon: 'ak47' }] },
      { id: 'second', round: 4, tick_start: 210, tick_end: 260, kills: [{ weapon: 'awp' }] },
    ],
  });
  assert.deepEqual(plays.map((play) => play.id), ['first', 'second']);
});

test('play context uses the source tick rate and only reports known headshot counts', () => {
  const [play] = planToPlays('job-1', { demo: { tickrate: 128 }, segments: [
    { id: 'r1', round: 1, tick_start: 1280, tick_end: 2816, kills: [{ weapon: 'ak47', headshot: true }, { weapon: 'ak47', headshot: false }] },
  ] });
  assert.equal(play.startSeconds, 10);
  assert.equal(play.endSeconds, 22);
  assert.equal(play.headshots, 1);
  const [unknown] = planToPlays('job-1', { segments: [
    { id: 'r1', round: 1, tick_start: 1280, tick_end: 2816, kills: [{ weapon: 'ak47' }] },
  ] });
  assert.equal(unknown.startSeconds, undefined);
  assert.equal(unknown.headshots, undefined);
});

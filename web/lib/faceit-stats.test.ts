import assert from 'node:assert/strict';
import test from 'node:test';
import type { FaceitMatch, FaceitMatchStats } from './api/faceit.ts';
import { summarizeFaceitMatches } from './faceit-stats.ts';

function match(result?: FaceitMatchStats['result'], values: Partial<FaceitMatchStats> = {}): FaceitMatch {
  return { id: 'match', room_url: 'https://www.faceit.com/en/cs2/room/match', score: {},
    stats: result === undefined ? undefined : { result, kills: 0, deaths: 0, assists: 0, ...values } };
}

test('missing or unknown results never count as defeats', () => {
  const summary = summarizeFaceitMatches([match('win'), match('loss'), match('unknown'), match()]);
  assert.equal(summary.winRate, 50);
  assert.equal(summary.wins, 1);
  assert.equal(summary.losses, 1);
  assert.equal(summary.unknown, 2);
});

test('an unavailable result has no win percentage instead of an invented zero', () => {
  assert.equal(summarizeFaceitMatches([match(), match('unknown')]).winRate, undefined);
  assert.equal(summarizeFaceitMatches([]).winRate, undefined);
});

test('averages keep real zeroes and exclude unavailable or non-finite measurements', () => {
  const summary = summarizeFaceitMatches([
    match('win', { kd_ratio: 2, adr: 100, headshots_percent: 80 }),
    match('loss', { kd_ratio: 0, adr: 0, headshots_percent: 0 }),
    match('unknown', { kd_ratio: Number.NaN, adr: Number.POSITIVE_INFINITY }),
    match(),
  ]);
  assert.equal(summary.kd, 1);
  assert.equal(summary.adr, 50);
  assert.equal(summary.headshots, 40);
  assert.equal(summarizeFaceitMatches([match()]).kd, undefined);
});

import test from 'node:test';
import assert from 'node:assert/strict';
import type { LibraryMatch } from '../api/library.ts';
import {
  actionBlocker,
  analysisVideoHref,
  defaultVideoPlayer,
  groupEvidenceByMatch,
  playsInRounds,
  roundListLabel,
  videoPlayers,
} from './analysis-video.ts';

const MATCH_A = '11111111-1111-4111-8111-111111111111';
const MATCH_B = '22222222-2222-4222-8222-222222222222';
const JOB = '33333333-3333-4333-8333-333333333333';
const TEAM = '44444444-4444-4444-8444-444444444444';

function match(): LibraryMatch {
  const player = (steamid64: string, team_key: 'ct-start' | 't-start', kills: number) => ({
    steamid64, name: steamid64, team_key, slot: 0, rounds: 20, kills, deaths: 10, assists: 0, damage: 0,
  });
  return {
    id: MATCH_A, demo_id: MATCH_B, map: 'de_mirage', played_at: '2026-10-01T00:00:00Z', source: 'faceit', tickrate: 64,
    duration_ticks: 1, round_count: 20, created_at: '', updated_at: '',
    teams: [
      { key: 'ct-start', start_side: 'CT', clan_name: 'A', team_id: null, assignment: 'none', score: 13 },
      { key: 't-start', start_side: 'T', clan_name: 'B', team_id: TEAM, assignment: 'auto', score: 7 },
    ],
    players: [player('1', 'ct-start', 30), player('2', 't-start', 5), player('3', 't-start', 12)],
  };
}

test('evidence groups by match in first-seen order with sorted unique rounds', () => {
  const groups = groupEvidenceByMatch([
    { match_id: MATCH_B, round: 9 },
    { match_id: MATCH_A, round: 4 },
    { match_id: MATCH_B, round: 2 },
    { match_id: MATCH_B, round: 9 },
  ]);
  assert.deepEqual(groups, [{ matchId: MATCH_B, rounds: [2, 9] }, { matchId: MATCH_A, rounds: [4] }]);
});

test('a POV is one round, the other actions take several, and each lands in its flow', () => {
  assert.equal(actionBlocker('pov', [3]), null);
  assert.ok(actionBlocker('pov', [3, 4]));
  assert.ok(actionBlocker('review', []));
  assert.equal(actionBlocker('short', [3, 4]), null);
  assert.equal(analysisVideoHref('review', JOB, [7, 3]), `/videos-largos/${JOB}?rondas=3%2C7`);
  assert.equal(analysisVideoHref('short', JOB, [5]), `/shorts/${JOB}?rondas=5`);
});

test('the default POV is the asked player, else the asked team top fragger', () => {
  const m = match();
  assert.deepEqual(videoPlayers(m, TEAM).map((p) => p.steamid64), ['3', '2', '1']);
  assert.equal(defaultVideoPlayer(m, { teamId: TEAM }), '3');
  assert.equal(defaultVideoPlayer(m, { teamId: TEAM, steamId: '1' }), '1');
  assert.equal(defaultVideoPlayer(m, { steamId: '9' }), '1');
  assert.equal(defaultVideoPlayer({ ...m, players: [] }, {}), null);
});

test('a Short from analysis opens with every play of the chosen rounds', () => {
  const plays = [{ id: 'a', round: 3 }, { id: 'b', round: 4 }, { id: 'c', round: 3 }];
  assert.deepEqual(playsInRounds(plays, [3]), ['a', 'c']);
  assert.deepEqual(playsInRounds(plays, [9]), []);
  assert.equal(roundListLabel([12, 3, 7]), '3, 7 y 12');
  assert.equal(roundListLabel([5]), '5');
});
